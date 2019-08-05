// Package ens implements a plugin that returns information held in the Ethereum Name Service.
package ens

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/labstack/gommon/log"
	ens "github.com/wealdtech/go-ens/v2"

	"github.com/miekg/dns"
)

// ENS is a plugin that returns information held in the Ethereum Name Service.
type ENS struct {
	Next             plugin.Handler
	Client           *ethclient.Client
	Registry         *ens.Registry
	EthLinkRoot      string
	IPFSGatewayAs    []string
	IPFSGatewayAAAAs []string
}

// IsAuthoritative checks if the ENS plugin is authoritative for a given domain
func (e ENS) IsAuthoritative(domain string) bool {
	// We consider ourselves authoritative if the domain has an SOA record in ENS
	rr, err := e.Query(domain, domain, dns.TypeNS, false)
	return err == nil && len(rr) > 0
}

// HasRecords checks if there are any records for a specific domain and name.
// This is used for wildcard eligibility
func (e ENS) HasRecords(domain string, name string) (bool, error) {
	resolver, err := ens.NewDNSResolver(e.Client, domain)
	if err != nil {
		return false, err
	}

	return resolver.HasRecords(name)
}

// Query queries a given domain/name/resource combination
func (e ENS) Query(domain string, name string, qtype uint16, do bool) ([]dns.RR, error) {
	log.Debugf("request type %d for name %s in domain %v", qtype, name, domain)
	ensDomain := strings.TrimSuffix(domain, ".")

	results := make([]dns.RR, 0)

	// Hard-coding some items for speed; these should be removed when the
	// relevant domain's records are present on-chain
	if ensDomain == "" || ensDomain == e.EthLinkRoot {
		return results, nil
	}

	if strings.HasSuffix(ensDomain, e.EthLinkRoot) {
		// This is a link request, using a secondary domain (e.g. eth.link) to redirect to .eth domains.
		// Map to a .eth domain and provide relevant (munged) information
		switch qtype {
		case dns.TypeNS:
			if !strings.HasPrefix(name, "_") {
				// TODO this data should be `ethRoot` DNS records but we don't have
				// control of the domain so hardcode it here
				result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN NS ns3.ethdns.xyz.", name))
				if err != nil {
					log.Warnf("error creating %s NS RR: %v", name, err)
				}
				results = append(results, result)
				result, err = dns.NewRR(fmt.Sprintf("%s 3600 IN NS ns4.ethdns.xyz.", name))
				if err != nil {
					log.Warnf("error creating %s NS RR: %v", name, err)
				}
				results = append(results, result)
			}
			return results, nil
		case dns.TypeSOA:
			if !strings.HasPrefix(name, "_") {
				// Create a synthetic SOA record
				now := time.Now()
				ser := ((now.Hour()*3600 + now.Minute()) * 100) / 86400
				dateStr := fmt.Sprintf("%04d%02d%02d%02d", now.Year(), now.Month(), now.Day(), ser)
				result, err := dns.NewRR(fmt.Sprintf("%s 10800 IN SOA ns3.ethdns.xyz. hostmaster.%s %s 3600 600 1209600 300", name, name, dateStr))
				if err != nil {
					log.Warnf("error creating %s NS RR: %v", name, err)
				}
				results = append(results, result)
				return results, nil
			}
		case dns.TypeTXT:
			txtRRSet, err := e.obtainTXTRRSet(name, domain)
			if err == nil && len(txtRRSet) != 0 {
				log.Infof("TXT record is %v", txtRRSet)
				// We have a TXT rrset; use it
				offset := 0
				for offset < len(txtRRSet) {
					var result dns.RR
					result, offset, err = dns.UnpackRR(txtRRSet, offset)
					if err == nil {
						results = append(results, result)
					}
				}
			}

			// Fetch content hash
			ethDomain := e.linkToEth(name)
			resolver, err := ens.NewResolver(e.Client, ethDomain)
			if err != nil {
				log.Warnf("error obtaining resolver: %v", err)
				return results, nil
			}

			address, err := resolver.Address()
			if err != nil {
				if err.Error() != "abi: unmarshalling empty output" {
					log.Warnf("error obtaining address: %v", err)
				}
			} else {
				if address != ens.UnknownAddress {
					result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN TXT \"a=%s\"", name, address.Hex()))
					if err != nil {
						log.Warnf("error creating address TXT RR: %v", err)
					}
					results = append(results, result)
				}
			}
			hash, err := resolver.Contenthash()
			if err != nil {
				if err.Error() != "abi: unmarshalling empty output" {
					log.Warnf("error obtaining content hash: %v", err)
				}
			} else {
				if len(hash) > 0 {
					result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN TXT \"contenthash=0x%x\"", name, hash))
					if err != nil {
						log.Warnf("error creating contenthash TXT RR: %v", err)
					} else {
						results = append(results, result)
					}
					// Also provide dnslink for compatibility with older IPFS gateways
					contentHash, err := ens.ContenthashToString(hash)
					if err != nil {
						log.Warnf("invalid content hash string: %v", err)
					} else {
						result, err = dns.NewRR(fmt.Sprintf("%s 3600 IN TXT \"dnslink=%s\"", name, contentHash))
						if err != nil {
							log.Warnf("error creating contenthash TXT RR: %v", err)
						} else {
							results = append(results, result)
						}
					}
				}
			}
		case dns.TypeA:
			// If the name is empty return our gateway
			if name == domain {
				for i := range e.IPFSGatewayAs {
					result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A %s", domain, e.IPFSGatewayAs[i]))
					if err != nil {
						log.Warnf("error creating %s A RR: %v", name, err)
					}
					results = append(results, result)
				}
			} else {
				// We want to return a default A rrset if the .eth resolver has a content
				// hash but not an A rrset
				aRRSet, err := e.obtainARRSet(name, domain)
				if err == nil && len(aRRSet) != 0 {
					// We have an A rrset; use it
					offset := 0
					for offset < len(aRRSet) {
						var result dns.RR
						result, offset, err = dns.UnpackRR(aRRSet, offset)
						if err == nil {
							results = append(results, result)
						}
					}
				} else {
					if len(e.IPFSGatewayAs) > 0 {
						contenthash, err := e.obtainContenthash(name, domain)
						if err == nil && len(contenthash) != 0 {
							// We have a content hash but no A record; use the default
							for i := range e.IPFSGatewayAs {
								result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A %s", name, e.IPFSGatewayAs[i]))
								if err != nil {
									log.Warnf("error creating %s A RR: %v", name, err)
								}
								results = append(results, result)
							}
						}
					}
				}
			}
		case dns.TypeAAAA:
			if name == domain {
				for i := range e.IPFSGatewayAAAAs {
					result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN AAAA %s", domain, e.IPFSGatewayAAAAs[i]))
					if err != nil {
						log.Warnf("error creating %s A RR: %v", name, err)
					}
					results = append(results, result)
				}
			} else {
				// We want to return a default A rrset if the .eth resolver has a content
				// We want to return a default AAAA rrset if the .eth resolver has a content
				// hash but not an AAAA rrset
				aaaaRRSet, err := e.obtainAAAARRSet(name, domain)
				if err == nil && len(aaaaRRSet) != 0 {
					// We have an AAAA rrset; use it
					offset := 0
					for offset < len(aaaaRRSet) {
						var result dns.RR
						result, offset, err = dns.UnpackRR(aaaaRRSet, offset)
						if err == nil {
							results = append(results, result)
						}
					}
				} else {
					if len(e.IPFSGatewayAAAAs) > 0 {
						contenthash, err := e.obtainContenthash(name, domain)
						if err == nil && len(contenthash) != 0 {
							// We have a content hash but no AAAA record; use the default
							for i := range e.IPFSGatewayAAAAs {
								result, err := dns.NewRR(fmt.Sprintf("%s 3600 IN AAAA %s", name, e.IPFSGatewayAAAAs[i]))
								if err != nil {
									log.Warnf("error creating %s AAAA RR: %v", name, err)
								}
								results = append(results, result)
							}
						}
					}
				}
			}
		}
		return results, nil
	}

	// Fetch whatever data we have on-chain for this RRset
	resolver, err := ens.NewDNSResolver(e.Client, ensDomain)
	if err != nil {
		if err.Error() != "no contract code at given address" {
			log.Warnf("error obtaining DNS resolver for %v: %v", ensDomain, err)
		}
		return results, err
	}

	data, err := resolver.Record(name, qtype)
	if err != nil {
		log.Warnf("error obtaining DNS record: %v", err)
		return results, err
	}

	offset := 0
	for offset < len(data) {
		var result dns.RR
		result, offset, err = dns.UnpackRR(data, offset)
		if err == nil {
			results = append(results, result)
		}
	}

	return results, err
}

// ServeDNS implements the plugin.Handler interface.
func (e ENS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	a := new(dns.Msg)
	a.SetReply(r)
	a.Compress = true
	a.Authoritative = true
	var result Result
	a.Answer, a.Ns, a.Extra, result = Lookup(e, state)
	switch result {
	case Success:
		state.SizeAndDo(a)
		w.WriteMsg(a)
		return 0, nil
	case NoData:
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	case NameError:
		a.Rcode = dns.RcodeNameError
	case ServerFailure:
		return dns.RcodeServerFailure, nil
	}
	// Unknown result...
	return dns.RcodeServerFailure, nil

}

func (e ENS) obtainARRSet(name string, domain string) ([]byte, error) {
	ethDomain := e.linkToEth(name)
	resolver, err := ens.NewDNSResolver(e.Client, ethDomain)
	if err != nil {
		if err.Error() == "no contract code at given address" ||
			strings.HasSuffix(err.Error(), " is not a DNS resolver contract") {
			return []byte{}, nil
		}
		log.Warnf("error obtaining resolver: %v", err)
		return []byte{}, err
	}

	return resolver.Record("", dns.TypeA)
}

func (e ENS) obtainAAAARRSet(name string, domain string) ([]byte, error) {
	ethDomain := e.linkToEth(name)
	resolver, err := ens.NewDNSResolver(e.Client, ethDomain)
	if err != nil {
		if err.Error() == "no contract code at given address" ||
			strings.HasSuffix(err.Error(), " is not a DNS resolver contract") {
			return []byte{}, nil
		}
		log.Warnf("error obtaining resolver: %v", err)
		return []byte{}, err
	}

	return resolver.Record("", dns.TypeAAAA)
}

func (e ENS) obtainContenthash(name string, domain string) ([]byte, error) {
	ethDomain := e.linkToEth(name)
	resolver, err := ens.NewResolver(e.Client, ethDomain)
	if err != nil {
		if err.Error() == "no contract code at given address" ||
			strings.HasSuffix(err.Error(), " is not a DNS resolver contract") {
			return []byte{}, nil
		}
		log.Warnf("error obtaining resolver: %v", err)
		return []byte{}, err
	}

	return resolver.Contenthash()
}

func (e ENS) obtainTXTRRSet(name string, domain string) ([]byte, error) {
	ethDomain := e.linkToEth(name)
	resolver, err := ens.NewDNSResolver(e.Client, ethDomain)
	if err != nil {
		if err.Error() == "no contract code at given address" ||
			strings.HasSuffix(err.Error(), " is not a DNS resolver contract") {
			return []byte{}, nil
		}
		log.Warnf("error obtaining resolver: %v", err)
		return []byte{}, err
	}

	return resolver.Record("", dns.TypeTXT)
}

// Name implements the Handler interface.
func (e ENS) Name() string { return "ens" }

// linkToEth obtains the .eth domain from the DNS domain
func (e ENS) linkToEth(domain string) string {
	ethDomain := strings.TrimSuffix(domain, ".")
	if e.EthLinkRoot != "" {
		ethDomain = fmt.Sprintf("%s.eth", strings.TrimSuffix(ethDomain, fmt.Sprintf(".%s", e.EthLinkRoot)))
	}
	return ethDomain
}
