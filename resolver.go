package main

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"strings"
	"sync"
	"time"
)

type Resolver struct {
}

func (r *Resolver) Lookup(net string, req *dns.Msg) (message *dns.Msg, err error) {
	c := &dns.Client{
		Net:          "tcp", //Always performance TCP dns query
		ReadTimeout:  r.Timeout(),
		WriteTimeout: r.Timeout(),
	}
	fmt.Println("Connect via : ", c.Net)

	qname := req.Question[0].Name

	res := make(chan *dns.Msg, 1)
	var wg sync.WaitGroup
	L := func(nameserver string) {
		defer wg.Done()
		r, rtt, err := c.Exchange(req, nameserver)
		if err != nil {
			fmt.Println("socket error on ", qname, nameserver)
			fmt.Println("error:", err.Error())
			return
		}
		// If SERVFAIL happen, should return immediately and try another upstream resolver.
		// However, other Error code like NXDOMAIN is an clear response stating
		// that it has been verified no such domain existas and ask other resolvers
		// would make no sense. See more about #20
		if r != nil && r.Rcode != dns.RcodeSuccess {
			fmt.Println("Failed to get an valid answer ", qname, nameserver)
			if r.Rcode == dns.RcodeServerFailure {
				return
			}
		} else {
			fmt.Println("resolv ", UnFqdn(qname), " on ", nameserver, c.Net, rtt)
		}
		select {
		case res <- r:
		default:
		}
	}

	ticker := time.NewTicker(time.Duration(200) * time.Millisecond)
	defer ticker.Stop()
	// Start lookup on each nameserver top-down, in every second
	for _, nameserver := range r.Nameservers() {
		wg.Add(1)
		go L(nameserver)
		// but exit early, if we have an answer
		select {
		case r := <-res:
			return r, nil
		case <-ticker.C:
			continue
		}
	}
	// wait for all the namservers to finish
	wg.Wait()
	select {
	case r := <-res:
		return r, nil
	default:
		return nil, errors.New(fmt.Sprintf("resolv failed on ", qname, " Via ", strings.Join(r.Nameservers(), "; "), net))
	}

}

// Namservers return the array of nameservers, with port number appended.
// '#' in the name is treated as port separator, as with dnsmasq.
func (r *Resolver) Nameservers() (ns []string) {
	ns = append(ns, "208.67.222.222:443")
	ns = append(ns, "208.67.220.220:443")
	return
}

func (r *Resolver) Timeout() time.Duration {
	return time.Duration(2) * time.Second
}
