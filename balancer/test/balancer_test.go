package test

import (
	"context"
	"testing"
	"time"

	"github.com/zhangel/go-framework.git/balancer/roundrobin"
	"github.com/zhangel/go-framework.git/registry"
)

func TestBenchmark(t *testing.T) {
	memoryRegistry, err := registry.NewMemoryServiceDiscovery()
	if err != nil {
		t.Fatal(err)
	}

	discovery, err := registry.NewDiscovery(memoryRegistry, roundrobin.NewProvider())
	if err != nil {
		t.Fatal(err)
	}

	if err := memoryRegistry.RegisterService("test.service", "192.168.0.1:2379", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	addr, err := discovery.Pick(context.Background(), "test.service")
	if err != nil {
		t.Fatal(err)
	} else if addr != "192.168.0.1:2379" {
		t.Fatal("addr.Addr != 192.168.0.1:2379")
	}

	if err := memoryRegistry.UnregisterService("test.service", "192.168.0.1:2379"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	addr, err = discovery.Pick(context.Background(), "test.service")
	if err == nil {
		t.Fatalf("Should got error because no address in registry, but got addr = %v", addr)
	}

	if err := memoryRegistry.RegisterService("test.service", "192.168.0.1:2379", nil); err != nil {
		t.Fatal(err)
	}

	if err := memoryRegistry.RegisterService("test.service", "192.168.0.1:2380", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	var addr1, addr2 string
	if addr, err = discovery.Pick(context.Background(), "test.service"); err != nil {
		t.Fatal(err)
	} else {
		addr1 = addr
	}

	if addr, err = discovery.Pick(context.Background(), "test.service"); err != nil {
		t.Fatal(err)
	} else {
		addr2 = addr
	}

	if addr1 == addr2 {
		t.Fatal("addr1 == addr2, but should not equal")
	} else if !((addr1 == "192.168.0.1:2379" && addr2 == "192.168.0.1:2380") || (addr2 == "192.168.0.1:2379" && addr1 == "192.168.0.1:2380")) {
		t.Fatal("addr unexpected")
	} else {
		t.Log("Addr1 =", addr1, ", Addr2 =", addr2)
	}
}
