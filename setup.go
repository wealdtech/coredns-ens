package ens

import (
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/ethereum/go-ethereum/ethclient"
	ens "github.com/wealdtech/go-ens/v2"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("ens", caddy.Plugin{
		ServerType: "dns",
		Action:     setupENS,
	})
}

func setupENS(c *caddy.Controller) error {
	connection, root, ipfsGatewayAs, ipfsGatewayAAAAs, err := ensParse(c)
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
			Next:             next,
			Client:           client,
			Registry:         registry,
			Root:             root,
			IPFSGatewayAs:    ipfsGatewayAs,
			IPFSGatewayAAAAs: ipfsGatewayAAAAs,
		}
	})

	return nil
}

func ensParse(c *caddy.Controller) (string, string, []string, []string, error) {
	var connection string
	var root string
	ipfsGatewayAs := make([]string, 0)
	ipfsGatewayAAAAs := make([]string, 0)

	// Obtain the root from the key
	root = c.Key
	idx := strings.Index(root, "://")
	if idx != -1 {
		root = root[idx+3:]
	}
	root = strings.TrimPrefix(root, "eth.")
	idx = strings.Index(root, ":")
	if idx != -1 {
		root = root[:idx]
	}
	root = strings.TrimSuffix(root, ".")

	c.Next()
	for c.NextBlock() {
		switch strings.ToLower(c.Val()) {
		case "connection":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", "", nil, nil, c.Errf("invalid connection; no value")
			}

			if len(args) > 1 {
				return "", "", nil, nil, c.Errf("invalid connection; multiple values")
			}
			connection = args[0]
		case "ipfsgatewaya":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", "", nil, nil, c.Errf("invalid IPFS gateway A; no value")
			}
			ipfsGatewayAs = make([]string, len(args))
			copy(ipfsGatewayAs, args)
		case "ipfsgatewayaaaa":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return "", "", nil, nil, c.Errf("invalid IPFS gateway AAAA; no value")
			}
			ipfsGatewayAAAAs = make([]string, len(args))
			copy(ipfsGatewayAAAAs, args)
		default:
			return "", "", nil, nil, c.Errf("unknown value %v", c.Val())
		}
	}
	if connection == "" {
		return "", "", nil, nil, c.Errf("no connection")
	}
	if root == "" {
		return "", "", nil, nil, c.Errf("no root")
	}
	return connection, root, ipfsGatewayAs, ipfsGatewayAAAAs, nil
}
