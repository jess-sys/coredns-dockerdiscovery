package dockerdiscovery

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"log"
)

func (dd *DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	var answers []dns.RR
	switch state.QType() {
	case dns.TypeCNAME, dns.TypeA:
		serviceInfo, err := dd.serviceInfoByHostname(state.QName())
		if err != nil {
			log.Println("[swarmdiscovery] Failed to get service info:", err)
			return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
		}
		if serviceInfo != nil && serviceInfo.hostnames != nil {
			//log.Printf("[swarmdiscovery] Found hostnames for service %s", serviceInfo.service.Spec.Name)
			//ip := net.ParseIP(serviceInfo.worker)
			//log.Printf("[swarmdiscovery] Found IP %s for service %s", ip.String(), serviceInfo.service.Spec.Name)
			answers = getAnswer(serviceInfo.worker, state.QName(), dd.ttl)
		} else {
			//log.Printf("[swarmdiscovery] No service found for query %s\n", state.QName())
		}
	}

	if len(answers) == 0 {
		//log.Printf("[swarmdiscovery] No answer found for query %s\n", state.QName())
		return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true
	m.Answer = answers

	state.SizeAndDo(m)
	m = state.Scrub(m)
	err := w.WriteMsg(m)
	if err != nil {
		log.Printf("[swarmdiscovery] Error while service DNS entry: %s", err.Error())
		return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
	}
	return dns.RcodeSuccess, nil
}

// func getAnswer(targetIp net.IP, hostname string, ttl uint32) []dns.RR {
func getAnswer(target string, hostname string, ttl uint32) []dns.RR {
	var answers []dns.RR
	record := new(dns.CNAME)
	record.Hdr = dns.RR_Header{
		Name:   dns.Fqdn(hostname),
		Rrtype: dns.TypeCNAME,
		Class:  dns.ClassINET,
		Ttl:    ttl,
	}
	record.Target = dns.Fqdn(target)
	answers = append(answers, record)
	return answers
}
