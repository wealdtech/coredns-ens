package ens

import (
	"strings"

	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// Result of a lookup
type Result int

const (
	// Success is a successful lookup.
	Success Result = iota
	// NameError indicates a nameerror
	NameError
	// Delegation indicates the lookup resulted in a delegation.
	Delegation
	// NoData indicates the lookup resulted in a NODATA.
	NoData
	// ServerFailure indicates a server failure during the lookup.
	ServerFailure
)

// Server is an interface defined by any plugin that wishes to serve
// authoritative records
type Server interface {
	// Query returns records for a specific domain, name, and resource type
	Query(domain string, qname string, qtype uint16, do bool) ([]dns.RR, error)

	// HasRecords checks if there are any records for a specific domain and name
	// This is used to check for wildcard eligibility
	HasRecords(domain string, qname string) (bool, error)

	// IsAuthoritative returns true if this server is authoritative for the
	// supplied domain
	IsAuthoritative(qdomain string) bool
}

// Obtain the lowest domain for which we are authoritative
func lowestAuthoritativeDomain(server Server, name string) string {
	parts := strings.Split(name, ".")
	var authoritativeDomain string
	testDomain := ""
	// Iterate backwards. Skip the last part, as it's always empty
	// (domains are dot-terminated).
	for i := len(parts) - 2; i >= 0; i-- {
		testDomain = parts[i] + "." + testDomain
		if server.IsAuthoritative(testDomain) {
			authoritativeDomain = testDomain
		}
	}
	return authoritativeDomain
}

// Lookup contains the logic required to move through A DNS hierarchy and
// gather the appropriate records
func Lookup(server Server, state request.Request) ([]dns.RR, []dns.RR, []dns.RR, Result) {
	qtype := state.QType()
	do := state.Do()

	answerRrs := make([]dns.RR, 0)
	authorityRrs := make([]dns.RR, 0)
	additionalRrs := make([]dns.RR, 0)

	// Work out the domain against which to query
	name := strings.ToLower(state.Name())
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	domain := lowestAuthoritativeDomain(server, name)
	if domain == "" {
		// We aren't authoritative for anything here
		return nil, nil, nil, NoData
	}

	// Look up parents of this name up to the domain to see if there are
	// any DNAME records. If so we take the first matching
	dnameName := name
	for {
		if dnameName == domain {
			break
		}
		dnameRrs, err := server.Query(domain, dnameName, dns.TypeDNAME, do)
		if err != nil {
			return nil, nil, nil, ServerFailure
		}
		if len(dnameRrs) > 0 {
			answerRrs = append(answerRrs, dnameRrs[0])
			synthName := substituteDNAME(name, dnameRrs[0].Header().Name, dnameRrs[0].(*dns.DNAME).Target)
			answerRrs = append(answerRrs, synthesizeCNAME(name, dnameRrs[0].(*dns.DNAME)))

			// RECURSE
			newReq := state.Req.Copy()
			newReq.Question[0].Name = synthName
			newState := request.Request{W: state.W, Req: newReq}
			dnameAnswerRrs, dnameAuthorityRrs, dnameAdditionalRrs, dnameResult := Lookup(server, newState)
			if dnameResult == Success {
				answerRrs = append(answerRrs, dnameAnswerRrs...)
				authorityRrs = append(authorityRrs, dnameAuthorityRrs...)
				additionalRrs = append(additionalRrs, dnameAdditionalRrs...)
			}
			return answerRrs, authorityRrs, additionalRrs, dnameResult
		}
		dotPos := strings.Index(dnameName, ".")
		if dotPos == -1 {
			break
		}
		if dotPos == len(dnameName) {
			dnameName = "."
		} else {
			dnameName = dnameName[dotPos+1:]
		}
		if dnameName == "" {
			break
		}
	}

	// Wildcard substitution
	if eligibleForWildcard(server, domain, name) {
		// We don't have any records for this name so try again using '*' instead of the actual name
		wildcardName := replaceWithAsteriskLabel(name)
		if wildcardName != name {
			newReq := state.Req.Copy()
			newReq.Question[0].Name = wildcardName
			newState := request.Request{W: state.W, Req: newReq}

			wildcardAnswerRrs, wildcardAuthorityRrs, wildcardAdditionalRrs, wildcardResult := Lookup(server, newState)
			if wildcardResult == Success {
				// Replace the wildcard results with original query results
				for _, answerRr := range wildcardAnswerRrs {
					if answerRr.Header().Name == wildcardName {
						answerRr.Header().Name = name
					}
					answerRrs = append(answerRrs, answerRr)
				}
				for _, authorityRr := range wildcardAuthorityRrs {
					if authorityRr.Header().Name == wildcardName {
						authorityRr.Header().Name = name
					}
					authorityRrs = append(authorityRrs, authorityRr)
				}
				for _, additionalRr := range wildcardAdditionalRrs {
					if additionalRr.Header().Name == wildcardName {
						additionalRr.Header().Name = name
					}
					additionalRrs = append(additionalRrs, additionalRr)
				}
			}
			return answerRrs, authorityRrs, additionalRrs, wildcardResult
		}
	}

	if qtype == dns.TypeNS {
		nsRrs, err := server.Query(domain, domain, dns.TypeNS, do)
		if err != nil {
			return nil, nil, nil, ServerFailure
		}
		// Nameserver records require additional processing
		if domain != name || len(nsRrs) == 0 {
			return nil, nil, nil, NoData
		}
		// Add glue for the NS records if present
		glueRrs := make([]dns.RR, 0)
		for i := 0; i < len(nsRrs); i++ {
			nameserver := nsRrs[i].(*dns.NS).Ns
			glueARrs, err := server.Query(domain, nameserver, dns.TypeA, do)
			if err == nil {
				glueRrs = append(glueRrs, glueARrs...)
			}
			glueAAAARrs, err := server.Query(domain, nameserver, dns.TypeAAAA, do)
			if err == nil {
				glueRrs = append(glueRrs, glueAAAARrs...)
			}
		}
		return nsRrs, nil, glueRrs, Success
	}

	// If we aren't asking for a CNAME then check for one to see if we need
	// to recurse
	if qtype != dns.TypeCNAME {
		cnameRrs, err := server.Query(domain, name, dns.TypeCNAME, do)
		if err != nil {
			return nil, nil, nil, ServerFailure
		}
		if len(cnameRrs) > 0 {
			// Found a CNAME; process it
			answerRrs = append(answerRrs, cnameRrs[0])
			cname := cnameRrs[0].(*dns.CNAME).Target
			// Create a new request
			newReq := state.Req.Copy()
			newReq.Question[0].Name = cname
			newReq.Question[0].Qtype = qtype
			newState := request.Request{W: state.W, Req: newReq}
			// Recurse with our new request
			cnameAnswerRrs, cnameAuthorityRrs, cnameAdditionalrs, cnameResult := Lookup(server, newState)
			if cnameResult == Success {
				answerRrs = append(answerRrs, cnameAnswerRrs...)
				authorityRrs = append(authorityRrs, cnameAuthorityRrs...)
				additionalRrs = append(additionalRrs, cnameAdditionalrs...)
			}
			return answerRrs, authorityRrs, additionalRrs, cnameResult
		}
	}
	// Fetch actual answer record(s)
	rrs, err := server.Query(domain, name, qtype, do)
	if err != nil {
		return nil, nil, nil, ServerFailure
	}
	if len(rrs) == 0 {
		return nil, nil, nil, NoData
	}
	answerRrs = append(answerRrs, rrs...)
	if len(answerRrs) == 0 {
		return answerRrs, authorityRrs, additionalRrs, NoData
	}

	if qtype == dns.TypeMX || qtype == dns.TypeSRV {
		// Add A and AAAA records to the answers provided where we can?
	}

	return answerRrs, authorityRrs, additionalRrs, Success
}
