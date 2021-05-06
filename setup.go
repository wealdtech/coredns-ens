package ens

import (
	"strings"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/ethereum/go-ethereum/ethclient"
	ens "github.com/wealdtech/go-ens/v3"
)

func init() {
	caddy.RegisterPlugin("ens", caddy.Plugin{
		ServerType: "dns",
		Action:     setupENS,
	})
}

func setupENS(c *caddy.Controller) error {
	connection, ethLinkNameServers, ipfsGatewayAs, ipfsGatewayAAAAs, err := ensParse(c)
	if err != nil {
		return plugin.Error("ens", err)
	}

	client, err := ethclient.Dial(connection)
	if err != nil {
		return plugin.Error("ens", err)
	}

	// Obtain the registry contract
	registry, err := ens.NewRegistry(client)
	if err != nil {
		return plugin.Error("ens", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return ENS{
			Next:               next,
			Client:             client,
			EthLinkNameServers: ethLinkNameServers,
			Registry:           registry,
			IPFSGatewayAs:      ipfsGatewayAs,
			IPFSGatewayAAAAs:   ipfsGatewayAAAAs,
		}
	})

	return nil
}

func ensParse(c *caddy.Controller) (string, []string, []string, []string, error) {
	var connection string
	ethLinkNameServers := make([]string, 0)
	ipfsGatewayAs := make([]string, 0)
	ipfsGatewayAAAAs := make([]string, 0)

	c.Next()
	for c.NextBlock() {
		switch strings.ToLower(c.Val()) {
		case "connection":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", nil, nil, nil, c.Errf("invalid connection; no value")
			}
			if len(args) > 1 {
				return "", nil, nil, nil, c.Errf("invalid connection; multiple values")
			}
			connection = args[0]
		case "ethlinknameservers":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", nil, nil, nil, c.Errf("invalid ethlinknameservers; no value")
			}
			ethLinkNameServers = make([]string, len(args))
			copy(ethLinkNameServers, args)
		case "ipfsgatewaya":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", nil, nil, nil, c.Errf("invalid IPFS gateway A; no value")
			}
			ipfsGatewayAs = make([]string, len(args))
			copy(ipfsGatewayAs, args)
		case "ipfsgatewayaaaa":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", nil, nil, nil, c.Errf("invalid IPFS gateway AAAA; no value")
			}
			ipfsGatewayAAAAs = make([]string, len(args))
			copy(ipfsGatewayAAAAs, args)
		default:
			return "", nil, nil, nil, c.Errf("unknown value %v", c.Val())
		}
	}
	if connection == "" {
		return "", nil, nil, nil, c.Errf("no connection")
	}
	if len(ethLinkNameServers) == 0 {
		return "", nil, nil, nil, c.Errf("no ethlinknameservers")
	}
	for i := range ethLinkNameServers {
		if !strings.HasSuffix(ethLinkNameServers[i], ".") {
			ethLinkNameServers[i] = ethLinkNameServers[i] + "."
		}
	}
	return connection, ethLinkNameServers, ipfsGatewayAs, ipfsGatewayAAAAs, nil
}
