package ens

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

type Record struct {
	domain   string
	class    uint16
	resource uint16
	record   string
}

type Zone struct {
	name    string
	records []Record
}

// Mock server
type MockServer struct {
	zones []Zone
}

var server = MockServer{
	zones: []Zone{
		{name: "example.com.", records: []Record{
			{"example.com.", dns.ClassINET, dns.TypeSOA, "example.com. 3600 IN SOA ns1.example.com. hostmaster.example.com. 2 19762 1800 1814400 14400"},
			{"example.com.", dns.ClassINET, dns.TypeNS, "example.com. 3600 IN NS ns1.example.com."},
			{"example.com.", dns.ClassINET, dns.TypeNS, "example.com. 3600 IN NS ns2.example.com."},
			{"www.example.com.", dns.ClassINET, dns.TypeCNAME, "www.example.com. 3600 IN CNAME example.com."},
			{"ns1.example.com.", dns.ClassINET, dns.TypeA, "ns1.example.com. 3600 IN A 1.1.1.1"},
			{"ns2.example.com.", dns.ClassINET, dns.TypeA, "ns2.example.com. 3600 IN A 1.1.1.2"},
			{"example.com.", dns.ClassINET, dns.TypeA, "example.com. 3600 IN A 1.1.2.1"},
			{"*.example.com.", dns.ClassINET, dns.TypeA, "*.example.com. 3600 IN A 1.1.2.2"},
			{"foo.example.com.", dns.ClassINET, dns.TypeDNAME, "foo.example.com. 3600 IN DNAME bar.example.com."},
			{"bar.example.com.", dns.ClassINET, dns.TypeA, "bar.example.com. 3600 IN A 1.1.2.3"},
			{"foo.bar.example.com.", dns.ClassINET, dns.TypeA, "foo.bar.example.com. 3600 IN A 1.1.2.4"},
		}},
		{name: "example.net.", records: []Record{}},
		{name: "mine.", records: []Record{}},
	},
}

func (m MockServer) IsAuthoritative(qname string) bool {
	for _, zone := range m.zones {
		if zone.name == qname {
			return true
		}
	}
	return false
}

func (m MockServer) Query(zone string, domain string, qtype uint16, do bool) ([]dns.RR, error) {
	results := make([]dns.RR, 0)
	for _, serverZone := range m.zones {
		if serverZone.name == zone {
			for _, serverRecord := range serverZone.records {
				if serverRecord.domain == domain && serverRecord.resource == qtype {
					rr, err := dns.NewRR(serverRecord.record)
					if err != nil {
						return nil, err
					}
					results = append(results, rr)
				}
			}
			break
		}
	}
	return results, nil
}

func (m MockServer) NumRecords(zone string, domain string) (uint16, error) {
	records := uint16(0)
	for _, serverZone := range m.zones {
		if serverZone.name == zone {
			for _, serverRecord := range serverZone.records {
				if serverRecord.domain == domain {
					records++
				}
			}
			break
		}
	}
	return records, nil
}

func (m MockServer) HasRecords(zone string, domain string) (bool, error) {
	numRecords, err := m.NumRecords(zone, domain)
	if err != nil {
		return false, err
	}
	return numRecords > 0, nil
}

// Helper to set up test records
func newRR(input string) dns.RR {
	rr, _ := dns.NewRR(input)
	return rr
}

func TestQuery(t *testing.T) {
	tests := []struct {
		zone     string
		domain   string
		resource uint16
		do       bool
		rrs      []dns.RR
	}{
		{"example.com.", "example.com.", dns.TypeNS, false, []dns.RR{
			newRR("example.com. 3600 IN NS ns1.example.com."),
			newRR("example.com. 3600 IN NS ns2.example.com."),
		}},
		{"example.net.", "example.net.", dns.TypeNS, false, []dns.RR{}},
	}
	for i, tt := range tests {
		rrs, err := server.Query(tt.zone, tt.domain, tt.resource, tt.do)
		if err != nil {
			t.Errorf("Test %d errored unexpectedly\n", i)
		}
		if len(rrs) != len(tt.rrs) {
			t.Errorf("Test %d returned %d records (expected %d)\n", i, len(rrs), len(tt.rrs))
		}
	}
}

var exampleComAuth = []dns.RR{
	test.NS("example.com.   3600    IN  NS  ns1.example.com."),
	test.NS("example.com.   3600    IN  NS  ns2.example.com."),
}

func TestLookup(t *testing.T) {
	tests := []test.Case{
		{
			Qname: "example.com.", Qtype: dns.TypeSOA,
			Answer: []dns.RR{
				test.SOA("example.com. 3600 IN SOA ns1.example.com. hostmaster.example.com. 2 19762 1800 1814400 14400"),
			},
			Ns: []dns.RR{
				test.NS("example.com.   3600    IN  NS  ns1.example.com."),
				test.NS("example.com.   3600    IN  NS  ns2.example.com."),
			},
		},
		{
			Qname: "example.com.", Qtype: dns.TypeNS,
			Answer: []dns.RR{
				test.NS("example.com.   3600    IN  NS  ns1.example.com."),
				test.NS("example.com.   3600    IN  NS  ns2.example.com."),
			},
			Extra: []dns.RR{
				test.A("ns1.example.com.  3600    IN  A   1.1.1.1"),
				test.A("ns2.example.com.  3600    IN  A   1.1.1.2"),
			},
		},
		{
			Qname: "example.com.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				test.A("example.com.  3600    IN  A   1.1.2.1"),
			},
			Ns: exampleComAuth,
		},
		{
			Qname: "www.example.com.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				test.A("example.com.  3600    IN  A   1.1.2.1"),
				test.CNAME("www.example.com.    3600    IN  CNAME   example.com."),
			},
			Ns: exampleComAuth,
		},
		{
			Qname: "wildcard.example.com.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				test.A("wildcard.example.com.  3600    IN  A   1.1.2.2"),
			},
			Ns: exampleComAuth,
		},
		{
			Qname: "foo.foo.example.com.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				test.A("foo.bar.example.com.  3600    IN  A   1.1.2.4"),
				test.DNAME("foo.example.com.  3600    IN  DNAME   bar.example.com."),
				test.CNAME("foo.foo.example.com.  3600    IN  CNAME   foo.bar.example.com."),
			},
			Ns: exampleComAuth,
		},
	}

	for _, tc := range tests {
		r := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})

		state := request.Request{W: rec, Req: r}
		a := new(dns.Msg)
		a.SetReply(r)
		a.Compress = true
		a.Authoritative = true
		a.Answer, a.Ns, a.Extra, _ = Lookup(server, state)

		state.SizeAndDo(a)
		rec.WriteMsg(a)

		resp := rec.Msg
		test.SortAndCheck(resp, tc)
	}
}

func TestAuthoritativeDomain(t *testing.T) {
	tests := []struct {
		name   string
		result string
	}{
		{"", ""},
		{".", ""},
		{"example.com.", "example.com."},
		{"sub.example.com.", "example.com."},
		{"foo.com.", ""},
		{"check.", ""},
		{"mine.", "mine."},
		{"my.mine.", "mine."},
	}

	for _, tt := range tests {
		result := lowestAuthoritativeDomain(server, tt.name)
		if tt.result != result {
			t.Errorf("Failure: %v => %v (expected %v)\n", tt.name, result, tt.result)
		}
	}
}
