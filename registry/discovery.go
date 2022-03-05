package registry

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	"github.com/zhangel/go-framework.git/balancer"
	"github.com/zhangel/go-framework.git/balancer/picker"
	"github.com/zhangel/go-framework.git/balancer/roundrobin"
	http2 "github.com/zhangel/go-framework.git/http"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/utils"
)

type AddressInfo struct {
	addr string
	tls  bool
}

func (s AddressInfo) Address() grpc.Address {
	return grpc.Address{Addr: s.addr}
}

func (s AddressInfo) Connected() bool {
	return true
}

func (s AddressInfo) Tls() bool {
	return s.tls
}

type mapPair struct {
	k string
	v string
}

type httpRuleMap struct {
	rules map[[md5.Size]byte]*httpRule
}

func newHttpRuleMap() *httpRuleMap {
	return &httpRuleMap{
		rules: make(map[[md5.Size]byte]*httpRule),
	}
}

func (s *httpRuleMap) AddHttpRule(registryHttpRule *HttpRule, addr picker.AddressInfo, provider balancer.Provider) error {
	if registryHttpRule == nil || addr == nil || provider == nil {
		return fmt.Errorf("no http rule or addr or provider found")
	}

	key, rule, err := s.generateKey(registryHttpRule)
	if err != nil {
		return err
	}

	if r, ok := s.rules[key]; ok {
		rule = r
	} else {
		s.rules[key] = rule
	}

	rule.addrs = append(rule.addrs, addr)
	rule.picker = picker.NewConcurrentPicker(provider().BuildPicker(rule.addrs))

	return nil
}

func (s *httpRuleMap) generateKey(registryHttpRule *HttpRule) ([md5.Size]byte, *httpRule, error) {
	keyBuilder := bytes.NewBuffer(nil)

	rule := &httpRule{
		patterns:  registryHttpRule.HttpPatterns,
		headerMap: registryHttpRule.HeaderMap,
	}

	patterns := rule.patterns[:]
	sort.Strings(patterns)
	keyBuilder.WriteString(strings.Join(patterns, ","))
	keyBuilder.WriteString("|")

	var headerPairs []*mapPair
	for _, h := range rule.headerMap {
		for k, v := range h {
			headerPairs = append(headerPairs, &mapPair{k: k, v: v})
		}
	}
	sort.SliceStable(headerPairs, func(i, j int) bool {
		return strings.Compare(headerPairs[i].k, headerPairs[j].k) < 0
	})
	for _, headerPair := range headerPairs {
		keyBuilder.WriteString(headerPair.k)
		keyBuilder.WriteString(",")
		keyBuilder.WriteString(headerPair.v)
		keyBuilder.WriteString(",")
	}
	keyBuilder.WriteString("|")

	var headerRegexPairs []*mapPair
	for _, r := range registryHttpRule.HeaderRegexMap {
		m, ms, err := utils.CreateHeaderRegexMap(r)
		if err != nil {
			return [md5.Size]byte{}, nil, fmt.Errorf("create regex header matcher failed, err = %v", err)
		}

		for k, v := range ms {
			headerRegexPairs = append(headerRegexPairs, &mapPair{k: k, v: v})
		}

		rule.headerRegexMap = append(rule.headerRegexMap, m)
	}
	sort.SliceStable(headerRegexPairs, func(i, j int) bool {
		return strings.Compare(headerRegexPairs[i].k, headerRegexPairs[j].k) < 0
	})
	for _, headerRegexPair := range headerRegexPairs {
		keyBuilder.WriteString(headerRegexPair.k)
		keyBuilder.WriteString(",")
		keyBuilder.WriteString(headerRegexPair.v)
		keyBuilder.WriteString(",")
	}

	return md5.Sum(keyBuilder.Bytes()), rule, nil
}

type httpRule struct {
	patterns       []string
	headerMap      []map[string]string
	headerRegexMap []map[string]*regexp.Regexp
	addrs          []picker.AddressInfo
	picker         picker.Picker
}

func (s *httpRule) HeaderMap() []map[string]string {
	return s.headerMap
}

func (s *httpRule) HeaderRegexMap() []map[string]*regexp.Regexp {
	return s.headerRegexMap
}

func (s *httpRule) Priority() int {
	if len(s.HeaderMap()) > 0 || len(s.HeaderRegexMap()) > 0 {
		return 1
	} else {
		return 0
	}
}

var (
	defaultDiscovery *discovery
	discoveryMutex   sync.RWMutex
)

type Discovery interface {
	Pick(ctx context.Context, serviceName string) (addr string, err error)
	PickWithInfo(ctx context.Context, serviceName string) (*AddressInfo, error)
	HttpPick(ctx context.Context, req *http.Request) (addr string, err error)
	HttpPickWithInfo(ctx context.Context, req *http.Request) (*AddressInfo, error)
	Down(addr string)
	Close()
}

type downAddrInfo struct {
	t          time.Time
	retryTimes int
}

type discovery struct {
	registry      Registry
	provider      balancer.Provider
	watchCanceler func()
	closeCh       chan struct{}

	httpMatcher           atomic.Value
	httpMatcherUpdateFlag uint32
	grpcEntries           sync.Map

	grpcServices      map[string][]Instance
	grpcServicesMutex sync.Mutex

	downList      map[string]*downAddrInfo
	downListMutex sync.RWMutex
}

func DefaultDiscovery() (Discovery, error) {
	discoveryMutex.RLock()
	if defaultDiscovery != nil {
		discoveryMutex.RUnlock()
		return defaultDiscovery, nil
	}
	discoveryMutex.RUnlock()

	discoveryMutex.Lock()
	defer discoveryMutex.Unlock()
	if defaultDiscovery != nil {
		return defaultDiscovery, nil
	}

	if discovery, err := NewDiscovery(DefaultRegistry(), roundrobin.NewProvider(roundrobin.WithAddressFilter(func(addr picker.AddressInfo) bool {
		return defaultDiscovery.isAddressAvailable(addr)
	}))); err != nil {
		return nil, err
	} else {
		defaultDiscovery = discovery
		return defaultDiscovery, nil
	}
}

func NewDiscovery(r Registry, provider balancer.Provider) (*discovery, error) {
	discovery := &discovery{
		registry:     r,
		provider:     provider,
		closeCh:      make(chan struct{}),
		grpcServices: make(map[string][]Instance),
		downList:     make(map[string]*downAddrInfo),
	}
	discovery.httpMatcher.Store(http2.NewPathMatcher())

	if err := discovery.initialize(); err != nil {
		return nil, err
	}

	if canceler, err := r.WatchAllServiceChanges(discovery); err != nil {
		return nil, err
	} else {
		discovery.watchCanceler = canceler
	}

	discovery.monitorHttpUpdate()
	discovery.downListMonitor()
	return discovery, nil
}

func (s *discovery) Close() {
	if s.watchCanceler != nil {
		s.watchCanceler()
	}
	close(s.closeCh)
}

func (s *discovery) initialize() error {
	entries, err := s.registry.ListAllService()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		s.updateGrpcEntries(entry.ServiceName, entry.Instances)
	}

	s.updateHttpMatcher(func() ([]Entry, error) { return entries, nil })
	return nil
}

func (s *discovery) OnUpdate(serviceName string, instances []Instance) {
	atomic.StoreUint32(&s.httpMatcherUpdateFlag, 1)
	s.updateGrpcEntries(serviceName, instances)
}

func (s *discovery) OnDelete(serviceName string, instances []Instance) {
	atomic.StoreUint32(&s.httpMatcherUpdateFlag, 1)
	s.deleteGrpcEntries(serviceName, instances)
}

func (s *discovery) updateHttpMatcher(getEntries func() ([]Entry, error)) {
	entries, err := getEntries()
	if err != nil {
		log.Warnf("discovery get all service entries failed, err = %v", err)
		return
	}

	newMatcher := http2.NewPathMatcher()
	ruleMap := newHttpRuleMap()

	for _, entry := range entries {
		for _, instance := range entry.Instances {
			jsonMeta, ok := instance.Meta.(string)
			if !ok {
				continue
			}

			registryMeta, err := UnmarshalRegistryMeta(jsonMeta)
			if err != nil {
				continue
			}

			if (registryMeta.Version < 2 && strings.HasPrefix(entry.ServiceName, http2.RegistryPrefix)) ||
				(registryMeta.Version == 2 && registryMeta.Type == RegistryType_Http) {
				rule := &HttpRule{
					HttpPatterns: registryMeta.HttpPatterns,
				}
				if err := ruleMap.AddHttpRule(rule, &AddressInfo{instance.Addr, registryMeta.TlsServer}, s.provider); err != nil {
					log.Errorf("addHttpRule failed, err = %v", err)
					continue
				}
			}

			if registryMeta.Version < 2 && !strings.HasPrefix(entry.ServiceName, http2.RegistryPrefix) {
				rule := &HttpRule{
					HttpPatterns: http2.RegistryWithPath([]string{"*"}, fmt.Sprintf("/%s/*", entry.ServiceName)),
				}

				if err := ruleMap.AddHttpRule(rule, &AddressInfo{instance.Addr, registryMeta.TlsServer}, s.provider); err != nil {
					log.Errorf("addHttpRule failed, err = %v", err)
					continue
				}
			}

			if registryMeta.Version == 3 && registryMeta.Type == RegistryType_Http {
				for _, rule := range registryMeta.HttpRules {
					if err := ruleMap.AddHttpRule(rule, &AddressInfo{instance.Addr, registryMeta.TlsServer}, s.provider); err != nil {
						log.Errorf("addHttpRule failed, err = %v", err)
						continue
					}
				}
			}

			if registryMeta.Version == 3 && registryMeta.Type == RegistryType_Http_Endpoint {
				rule := &HttpRule{
					HttpPatterns: http2.RegistryWithPath([]string{"*"}, fmt.Sprintf("/%s/*", strings.TrimSuffix(entry.ServiceName, ".http.endpoint"))),
				}

				if err := ruleMap.AddHttpRule(rule, &AddressInfo{instance.Addr, registryMeta.TlsServer}, s.provider); err != nil {
					log.Errorf("addHttpRule failed, err = %v", err)
					continue
				}
			}
		}
	}

	for _, rule := range ruleMap.rules {
		for _, pat := range rule.patterns {
			if err := newMatcher.AddPattern("", pat, rule); err != nil {
				log.Errorf("discovery addPattern failed, pattern = %s, err = %v", pat, err)
			}
		}
	}

	s.httpMatcher.Store(newMatcher)
}

func (s *discovery) monitorHttpUpdate() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if atomic.CompareAndSwapUint32(&s.httpMatcherUpdateFlag, 1, 0) {
					s.updateHttpMatcher(s.registry.ListAllService)
					s.downListMutex.Lock()
					s.downList = map[string]*downAddrInfo{}
					s.downListMutex.Unlock()
				}
			case <-s.closeCh:
				return
			}
		}
	}()
}

func (s *discovery) updateGrpcEntries(serviceName string, instances []Instance) {
	if len(instances) == 0 {
		return
	}

	s.grpcServicesMutex.Lock()
	defer s.grpcServicesMutex.Unlock()

	var addrs []picker.AddressInfo
	instances = append(instances, s.grpcServices[serviceName]...)
	for _, instance := range instances {
		jsonMeta, ok := instance.Meta.(string)
		if !ok {
			addrs = append(addrs, &AddressInfo{instance.Addr, false})
			continue
		}

		registryMeta, err := UnmarshalRegistryMeta(jsonMeta)
		if err != nil {
			addrs = append(addrs, &AddressInfo{instance.Addr, false})
			continue
		}

		if registryMeta.Version < 2 && strings.HasPrefix(serviceName, http2.RegistryPrefix) ||
			registryMeta.Version >= 2 && registryMeta.Type == RegistryType_Http {
			continue
		}

		addrs = append(addrs, &AddressInfo{instance.Addr, registryMeta.TlsServer})
	}
	s.grpcEntries.Store(serviceName, picker.NewConcurrentPicker(s.provider().BuildPicker(addrs)))
	s.grpcServices[serviceName] = instances
}

func (s *discovery) deleteGrpcEntries(serviceName string, instances []Instance) {
	if len(instances) == 0 {
		return
	}

	s.grpcServicesMutex.Lock()
	defer s.grpcServicesMutex.Unlock()

	var remains []Instance
	prevInstances := s.grpcServices[serviceName]
	for _, prevInstance := range prevInstances {
		del := false
		for _, instance := range instances {
			if instance.Addr == prevInstance.Addr {
				del = true
				break
			}
		}

		if !del {
			remains = append(remains, prevInstance)
		}
	}

	var addrs []picker.AddressInfo
	for _, instance := range remains {
		jsonMeta, ok := instance.Meta.(string)
		if !ok {
			addrs = append(addrs, &AddressInfo{instance.Addr, false})
			continue
		}

		registryMeta, err := UnmarshalRegistryMeta(jsonMeta)
		if err != nil {
			addrs = append(addrs, &AddressInfo{instance.Addr, false})
			continue
		}

		if registryMeta.Version < 2 && strings.HasPrefix(serviceName, http2.RegistryPrefix) ||
			registryMeta.Version >= 2 && registryMeta.Type == RegistryType_Http {
			continue
		}

		addrs = append(addrs, &AddressInfo{instance.Addr, registryMeta.TlsServer})
	}
	if len(addrs) > 0 {
		s.grpcEntries.Store(serviceName, picker.NewConcurrentPicker(s.provider().BuildPicker(addrs)))
	} else {
		s.grpcEntries.Delete(serviceName)
	}

	s.grpcServices[serviceName] = remains
}

func (s *discovery) Pick(ctx context.Context, serviceName string) (string, error) {
	addrInfo, err := s.PickWithInfo(ctx, serviceName)
	if err != nil {
		return "", err
	}

	return addrInfo.addr, nil
}

func (s *discovery) PickWithInfo(ctx context.Context, serviceName string) (addr *AddressInfo, err error) {
	defer func() {
		log.Tracef("discovery:Pick, serviceName = %s, addrInfo = %v, err = %v", serviceName, addr, err)
	}()

	p, ok := s.grpcEntries.Load(serviceName)
	if !ok {
		return nil, fmt.Errorf("discovery Pick, no registry entry of %q found", serviceName)
	}

	addrInfo, err := p.(picker.Picker).Pick(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovery Pick, pick address failed, err = %v", err)
	}

	return addrInfo.(*AddressInfo), nil
}

func (s *discovery) HttpPick(ctx context.Context, req *http.Request) (string, error) {
	addrInfo, err := s.HttpPickWithInfo(ctx, req)
	if err != nil {
		return "", err
	}

	return addrInfo.addr, nil
}

func (s *discovery) HttpPickWithInfo(ctx context.Context, req *http.Request) (addr *AddressInfo, err error) {
	defer func() {
		log.Tracef("discovery:HttpPick, method = %s, path = %s, addrInfo = %v, err = %v", req.Method, req.URL.Path, addr, err)
	}()

	httpMatcher, ok := s.httpMatcher.Load().(*http2.PathMatcher)
	if !ok || httpMatcher == nil {
		return nil, fmt.Errorf("discovery pickWithHttpPattern, httpMatcher not ready")
	}

	meta, err := httpMatcher.MatchWithMeta(req, http2.EstimateHeader)
	if err != nil || len(meta) == 0 {
		return nil, fmt.Errorf("discovery pickWithHttpPattern no matched pattern found, err = %v", err)
	}

	rand.Seed(time.Now().UnixNano())
	addrInfo, err := meta[rand.Intn(len(meta))].(*httpRule).picker.Pick(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovery pickWithHttpPattern, pick address failed, err = %v", err)
	}

	return addrInfo.(*AddressInfo), nil
}

func (s *discovery) Down(addr string) {
	s.downListMutex.Lock()
	defer s.downListMutex.Unlock()

	log.Warnf("discovery: Address %q is down", addr)
	s.downList[addr] = &downAddrInfo{
		t:          time.Now(),
		retryTimes: 0,
	}
}

func (s *discovery) isAddressAvailable(addr picker.AddressInfo) bool {
	s.downListMutex.RLock()
	defer s.downListMutex.RUnlock()

	_, ok := s.downList[addr.Address().Addr]
	return !ok
}

func (s *discovery) downListMonitor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case t := <-ticker.C:
				s.downListMutex.RLock()
				for k, v := range s.downList {
					if t.Sub(v.t) >= 1*time.Minute && v.retryTimes < 5 {
						v.retryTimes++
						go s.checkDownAddr(k, v.retryTimes)
					}
				}
				s.downListMutex.RUnlock()
			case <-s.closeCh:
				return
			}
		}
	}()
}

func (s *discovery) checkDownAddr(addr string, retryTime int) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if conn != nil {
		_ = conn.Close()
	}

	if err == nil {
		s.downListMutex.Lock()
		delete(s.downList, addr)
		log.Warnf("discovery: Address %q is up", addr)
		s.downListMutex.Unlock()
	} else {
		log.Warnf("discovery: Healthy check %q [%d] failed, err = %v", addr, retryTime, err)
	}
}
