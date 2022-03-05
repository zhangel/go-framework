package balancer

import (
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/naming"

	"github.com/zhangel/go-framework.git/balancer/internal"
	"github.com/zhangel/go-framework.git/declare"
	"github.com/zhangel/go-framework.git/plugin"
)

var (
	Plugin = declare.PluginType{Name: "balancer"}

	defaultBalancerBuilder Builder
)

type Provider internal.Provider

type Builder interface {
	Build(resolver naming.Resolver) grpc.Balancer
}

type BuilderFunc func(resolver naming.Resolver) grpc.Balancer

func (s BuilderFunc) Build(resolver naming.Resolver) grpc.Balancer {
	return s(resolver)
}

func DefaultBalancerBuilder() Builder {
	if defaultBalancerBuilder != nil {
		return defaultBalancerBuilder
	}

	var provider Provider
	err := plugin.CreatePlugin(Plugin, &provider)
	if err != nil {
		log.Fatalf("[ERROR] Create balancer plugin failed, err = %s.\n", err)
	}

	if provider == nil {
		defaultBalancerBuilder = BuilderFunc(func(resolver naming.Resolver) grpc.Balancer { return grpc.RoundRobin(resolver) })
	} else {
		defaultBalancerBuilder, _ = BuilderWithProvider(provider)
	}
	return defaultBalancerBuilder
}

func BuilderWithName(name string) (Builder, error) {
	var provider Provider
	if err := plugin.CreatePluginWithName(Plugin, name, &provider); err != nil {
		return nil, fmt.Errorf("create balancer plugin failed, err = %+v", err)
	}

	return BuilderWithProvider(provider)
}

func BuilderWithProvider(provider Provider) (Builder, error) {
	return BuilderFunc(func(resolver naming.Resolver) grpc.Balancer {
		builder, _ := internal.NewBalancerWithPicker(resolver, provider())
		return builder
	}), nil
}
