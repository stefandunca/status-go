package commands

import (
	"database/sql"
	"strconv"

	"github.com/status-im/status-go/services/connector/chainutils"
	persistence "github.com/status-im/status-go/services/connector/database"
	walletCommon "github.com/status-im/status-go/services/wallet/common"
)

type ChainIDCommand struct {
	NetworkManager NetworkManagerInterface
	Db             *sql.DB
}

func (c *ChainIDCommand) Execute(request RPCRequest) (interface{}, error) {
	err := request.Validate()
	if err != nil {
		return "", err
	}

	dApp, err := persistence.SelectDAppByUrl(c.Db, request.URL)
	if err != nil {
		return "", err
	}

	if dApp == nil {
		defaultChainID, err := chainutils.GetDefaultChainID(c.NetworkManager)
		if err != nil {
			return "", err
		}

		chainId, err := chainutils.GetHexChainID(strconv.FormatUint(defaultChainID, 16))
		if err != nil {
			return "", err
		}

		return chainId, nil
	}

	chainId, err := chainutils.GetHexChainID(walletCommon.ChainID(dApp.ChainID).String())
	if err != nil {
		return "", err
	}

	return chainId, nil
}
