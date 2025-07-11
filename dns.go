package dockerdiscovery

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"log"
	"net"
)

func (dd *DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	switch state.QType() {
	case dns.TypeA, dns.TypeCNAME:
		svc, err := dd.serviceInfoByHostname(state.QName())
		if err != nil {
			log.Printf("[swarmdiscovery] lookup error: %v", err)
			return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
		}
		if svc != nil {
			// build either CNAME or A (or CNAME‐for‐A) answers
			answers := getAnswer(state.QType(), svc.worker, svc.hostnames, dd.ttl)
			if len(answers) == 0 {
				// nothing known → hand off
				return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
			}

			// write our partial answer
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = false
			msg.Answer = answers
			w.WriteMsg(msg)

			// if the client asked for A, but we only gave them a CNAME,
			// rewrite the question to the target and let the next plugin resolve it
			if state.QType() == dns.TypeA {
				if _, isCname := answers[0].(*dns.CNAME); isCname {
					// mutate the question in place
					r.Question[0].Name = dns.Fqdn(svc.worker)
					return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
				}
			}

			return dns.RcodeSuccess, nil
		}
	}

	// all other cases, or no service found
	return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
}

func getAnswer(qtype uint16, target string, hostnames []string, ttl uint32) []dns.RR {
	var answers []dns.RR
	fqTarget := dns.Fqdn(target)

	for _, h := range hostnames {
		fqHost := dns.Fqdn(h)
		switch qtype {
		case dns.TypeCNAME:
			answers = append(answers, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: fqHost, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: fqTarget,
			})

		case dns.TypeA:
			// if the target is an IP, return an A
			if ip := net.ParseIP(target); ip != nil {
				answers = append(answers, &dns.A{
					Hdr: dns.RR_Header{Name: fqHost, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   ip,
				})
			} else {
				// otherwise emit a CNAME and let the next plugin chase down the A
				answers = append(answers, &dns.CNAME{
					Hdr:    dns.RR_Header{Name: fqHost, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
					Target: fqTarget,
				})
			}
		}
	}

	return answers
}
