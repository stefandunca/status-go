package communities

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"go.uber.org/zap"

	maps "golang.org/x/exp/maps"
	slices "golang.org/x/exp/slices"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/status-im/status-go/protocol/ens"
	"github.com/status-im/status-go/protocol/protobuf"
	walletcommon "github.com/status-im/status-go/services/wallet/common"
	"github.com/status-im/status-go/services/wallet/thirdparty"
)

type PermissionChecker interface {
	CheckPermissionToJoin(*Community, []gethcommon.Address) (*CheckPermissionToJoinResponse, error)
	CheckPermissions(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool) (*CheckPermissionsResponse, error)
	CheckPermissionsWithPreFetchedData(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool, collectiblesOwners CollectiblesOwners) (*CheckPermissionsResponse, error)
}

type DefaultPermissionChecker struct {
	tokenManager        TokenManager
	collectiblesManager CollectiblesManager
	ensVerifier         *ens.Verifier

	logger *zap.Logger
}

type PreParsedPermissionsData struct {
	Erc721TokenRequirements map[uint64]map[string]*protobuf.TokenCriteria
	Erc20TokenAddresses     []gethcommon.Address
	Erc20ChainIDsMap        map[uint64]bool
	Erc721ChainIDsMap       map[uint64]bool
}

type PreParsedCommunityPermissionsData struct {
	*PreParsedPermissionsData
	Permissions []*CommunityTokenPermission
}

func (p *DefaultPermissionChecker) getOwnedENS(addresses []gethcommon.Address) ([]string, error) {
	ownedENS := make([]string, 0)
	if p.ensVerifier == nil {
		p.logger.Warn("no ensVerifier configured for communities manager")
		return ownedENS, nil
	}
	for _, address := range addresses {
		name, err := p.ensVerifier.ReverseResolve(address)
		if err != nil && err.Error() != "not a resolver" {
			return ownedENS, err
		}
		if name != "" {
			ownedENS = append(ownedENS, name)
		}
	}
	return ownedENS, nil
}

type collectiblesBalancesGetter = func(ctx context.Context, chainID walletcommon.ChainID, ownerAddress gethcommon.Address, contractAddresses []gethcommon.Address) (thirdparty.TokenBalancesPerContractAddress, error)

func (p *DefaultPermissionChecker) getOwnedERC721Tokens(walletAddresses []gethcommon.Address, tokenRequirements map[uint64]map[string]*protobuf.TokenCriteria, chainIDs []uint64, getCollectiblesBalances collectiblesBalancesGetter) (CollectiblesByChain, error) {
	if p.collectiblesManager == nil {
		return nil, errors.New("no collectibles manager")
	}

	ctx := context.Background()

	ownedERC721Tokens := make(CollectiblesByChain)

	for chainID, erc721Tokens := range tokenRequirements {

		skipChain := true
		for _, cID := range chainIDs {
			if chainID == cID {
				skipChain = false
			}
		}

		if skipChain {
			continue
		}

		contractAddresses := make([]gethcommon.Address, 0)
		for contractAddress := range erc721Tokens {
			contractAddresses = append(contractAddresses, gethcommon.HexToAddress(contractAddress))
		}

		if _, exists := ownedERC721Tokens[chainID]; !exists {
			ownedERC721Tokens[chainID] = make(map[gethcommon.Address]thirdparty.TokenBalancesPerContractAddress)
		}

		for _, owner := range walletAddresses {
			balances, err := getCollectiblesBalances(ctx, walletcommon.ChainID(chainID), owner, contractAddresses)
			if err != nil {
				p.logger.Info("couldn't fetch owner assets", zap.Error(err))
				return nil, err
			}
			ownedERC721Tokens[chainID][owner] = balances
		}
	}
	return ownedERC721Tokens, nil
}

func (p *DefaultPermissionChecker) accountChainsCombinationToMap(combinations []*AccountChainIDsCombination) map[gethcommon.Address][]uint64 {
	result := make(map[gethcommon.Address][]uint64)
	for _, combination := range combinations {
		result[combination.Address] = combination.ChainIDs
	}
	return result
}

// merge valid combinations w/o duplicates
func (p *DefaultPermissionChecker) MergeValidCombinations(left, right []*AccountChainIDsCombination) []*AccountChainIDsCombination {

	leftMap := p.accountChainsCombinationToMap(left)
	rightMap := p.accountChainsCombinationToMap(right)

	// merge maps, result in left map
	for k, v := range rightMap {
		if _, exists := leftMap[k]; !exists {
			leftMap[k] = v
			continue
		} else {
			// append chains which are new
			chains := leftMap[k]
			for _, chainID := range v {
				if !slices.Contains(chains, chainID) {
					chains = append(chains, chainID)
				}
			}
			leftMap[k] = chains
		}
	}

	result := []*AccountChainIDsCombination{}
	for k, v := range leftMap {
		result = append(result, &AccountChainIDsCombination{
			Address:  k,
			ChainIDs: v,
		})
	}

	return result
}

func (p *DefaultPermissionChecker) CheckPermissionToJoin(community *Community, addresses []gethcommon.Address) (*CheckPermissionToJoinResponse, error) {
	becomeAdminPermissions := community.TokenPermissionsByType(protobuf.CommunityTokenPermission_BECOME_ADMIN)
	becomeMemberPermissions := community.TokenPermissionsByType(protobuf.CommunityTokenPermission_BECOME_MEMBER)
	becomeTokenMasterPermissions := community.TokenPermissionsByType(protobuf.CommunityTokenPermission_BECOME_TOKEN_MASTER)

	adminOrTokenMasterPermissionsToJoin := append(becomeAdminPermissions, becomeTokenMasterPermissions...)

	allChainIDs, err := p.tokenManager.GetAllChainIDs()
	if err != nil {
		return nil, err
	}

	accountsAndChainIDs := combineAddressesAndChainIDs(addresses, allChainIDs)

	// Check becomeMember and (admin & token master) permissions separately.
	becomeMemberPermissionsResponse, err := p.checkPermissionsOrDefault(becomeMemberPermissions, accountsAndChainIDs)
	if err != nil {
		return nil, err
	}

	if len(adminOrTokenMasterPermissionsToJoin) <= 0 {
		return becomeMemberPermissionsResponse, nil
	}
	// If there are any admin or token master permissions, combine result.
	preParsedPermissions := preParsedCommunityPermissionsData(adminOrTokenMasterPermissionsToJoin)
	var adminOrTokenPermissionsResponse *CheckPermissionsResponse

	if community.IsControlNode() {
		adminOrTokenPermissionsResponse, err = p.CheckPermissions(preParsedPermissions, accountsAndChainIDs, false)
	} else {
		adminOrTokenPermissionsResponse, err = p.CheckCachedPermissions(preParsedPermissions, accountsAndChainIDs, false)
	}
	if err != nil {
		return nil, err
	}

	mergedPermissions := make(map[string]*PermissionTokenCriteriaResult)
	maps.Copy(mergedPermissions, becomeMemberPermissionsResponse.Permissions)
	maps.Copy(mergedPermissions, adminOrTokenPermissionsResponse.Permissions)

	mergedCombinations := p.MergeValidCombinations(becomeMemberPermissionsResponse.ValidCombinations, adminOrTokenPermissionsResponse.ValidCombinations)

	combinedResponse := &CheckPermissionsResponse{
		Satisfied:         becomeMemberPermissionsResponse.Satisfied || adminOrTokenPermissionsResponse.Satisfied,
		Permissions:       mergedPermissions,
		ValidCombinations: mergedCombinations,
	}

	return combinedResponse, nil
}

func (p *DefaultPermissionChecker) checkPermissionsOrDefault(permissions []*CommunityTokenPermission, accountsAndChainIDs []*AccountChainIDsCombination) (*CheckPermissionsResponse, error) {
	if len(permissions) == 0 {
		// There are no permissions to join on this community at the moment,
		// so we reveal all accounts + all chain IDs
		response := &CheckPermissionsResponse{
			Satisfied:         true,
			Permissions:       make(map[string]*PermissionTokenCriteriaResult),
			ValidCombinations: accountsAndChainIDs,
		}
		return response, nil
	}

	preParsedPermissions := preParsedCommunityPermissionsData(permissions)
	return p.CheckCachedPermissions(preParsedPermissions, accountsAndChainIDs, false)
}

type ownedERC721TokensGetter = func(walletAddresses []gethcommon.Address, tokenRequirements map[uint64]map[string]*protobuf.TokenCriteria, chainIDs []uint64) (CollectiblesByChain, error)
type balancesByChainGetter = func(ctx context.Context, accounts, tokens []gethcommon.Address, chainIDs []uint64) (BalancesByChain, error)

func (p *DefaultPermissionChecker) checkTokenRequirement(
	tokenRequirement *protobuf.TokenCriteria,
	accounts []gethcommon.Address, ownedERC20TokenBalances BalancesByChain, ownedERC721Tokens CollectiblesByChain,
	accountsChainIDsCombinations map[gethcommon.Address]map[uint64]bool,
) (TokenRequirementResponse, error) {
	tokenRequirementResponse := TokenRequirementResponse{TokenCriteria: tokenRequirement}

	switch tokenRequirement.Type {

	case protobuf.CommunityTokenType_ERC721:

		if len(ownedERC721Tokens) == 0 {
			return tokenRequirementResponse, nil
		}

		// Limit NFTs count to uint32
		requiredCount, err := strconv.ParseUint(tokenRequirement.AmountInWei, 10, 32)
		if err != nil {
			return tokenRequirementResponse, fmt.Errorf("invalid ERC721 amount: %s", tokenRequirement.AmountInWei)
		}
		accumulatedCount := uint64(0)

		for chainID, addressStr := range tokenRequirement.ContractAddresses {
			contractAddress := gethcommon.HexToAddress(addressStr)
			if _, exists := ownedERC721Tokens[chainID]; !exists || len(ownedERC721Tokens[chainID]) == 0 {
				continue
			}

			for account := range ownedERC721Tokens[chainID] {
				if _, exists := ownedERC721Tokens[chainID][account]; !exists {
					continue
				}

				tokenBalances := ownedERC721Tokens[chainID][account][contractAddress]
				accumulatedCount += uint64(len(tokenBalances))

				if len(tokenBalances) > 0 {
					// 'account' owns some TokenID owned from contract 'address'
					if _, exists := accountsChainIDsCombinations[account]; !exists {
						accountsChainIDsCombinations[account] = make(map[uint64]bool)
					}

					// account has balance > 0 on this chain for this token, so let's add it the chain IDs
					accountsChainIDsCombinations[account][chainID] = true

					if len(tokenRequirement.TokenIds) == 0 {
						// no specific tokenId of this collection is needed

						if accumulatedCount >= requiredCount {
							tokenRequirementResponse.Satisfied = true
							return tokenRequirementResponse, nil
						}
					}

					for _, tokenID := range tokenRequirement.TokenIds {
						tokenIDBigInt := new(big.Int).SetUint64(tokenID)

						for _, asset := range tokenBalances {
							if asset.TokenID.Cmp(tokenIDBigInt) == 0 && asset.Balance.Sign() > 0 {
								tokenRequirementResponse.Satisfied = true
								return tokenRequirementResponse, nil
							}
						}
					}
				}
			}
		}

	case protobuf.CommunityTokenType_ERC20:

		if len(ownedERC20TokenBalances) == 0 {
			return tokenRequirementResponse, nil
		}

		accumulatedBalance := new(big.Int)

	chainIDLoopERC20:
		for chainID, address := range tokenRequirement.ContractAddresses {
			if _, exists := ownedERC20TokenBalances[chainID]; !exists || len(ownedERC20TokenBalances[chainID]) == 0 {
				continue chainIDLoopERC20
			}
			contractAddress := gethcommon.HexToAddress(address)
			for account := range ownedERC20TokenBalances[chainID] {
				if _, exists := ownedERC20TokenBalances[chainID][account][contractAddress]; !exists {
					continue
				}

				value := ownedERC20TokenBalances[chainID][account][contractAddress]

				if _, exists := accountsChainIDsCombinations[account]; !exists {
					accountsChainIDsCombinations[account] = make(map[uint64]bool)
				}

				if value.ToInt().Cmp(big.NewInt(0)) > 0 {
					// account has balance > 0 on this chain for this token, so let's add it the chain IDs
					accountsChainIDsCombinations[account][chainID] = true
				}

				// check if adding current chain account balance to accumulated balance
				// satisfies required amount
				prevBalance := accumulatedBalance
				accumulatedBalance.Add(prevBalance, value.ToInt())

				requiredAmount, success := new(big.Int).SetString(tokenRequirement.AmountInWei, 10)
				if !success {
					return tokenRequirementResponse, fmt.Errorf("amountInWeis value is incorrect: %s", tokenRequirement.AmountInWei)
				}

				if accumulatedBalance.Cmp(requiredAmount) != -1 {
					tokenRequirementResponse.Satisfied = true
					return tokenRequirementResponse, nil
				}
			}
		}

	case protobuf.CommunityTokenType_ENS:

		for _, account := range accounts {
			ownedENSNames, err := p.getOwnedENS([]gethcommon.Address{account})
			if err != nil {
				return tokenRequirementResponse, err
			}

			if _, exists := accountsChainIDsCombinations[account]; !exists {
				accountsChainIDsCombinations[account] = make(map[uint64]bool)
			}

			if !strings.HasPrefix(tokenRequirement.EnsPattern, "*.") {
				for _, ownedENS := range ownedENSNames {
					if ownedENS == tokenRequirement.EnsPattern {
						accountsChainIDsCombinations[account][walletcommon.EthereumMainnet] = true
						tokenRequirementResponse.Satisfied = true
						return tokenRequirementResponse, nil
					}
				}
			} else {
				parentName := tokenRequirement.EnsPattern[2:]
				for _, ownedENS := range ownedENSNames {
					if strings.HasSuffix(ownedENS, parentName) {
						accountsChainIDsCombinations[account][walletcommon.EthereumMainnet] = true
						tokenRequirementResponse.Satisfied = true
						return tokenRequirementResponse, nil
					}
				}
			}
		}

	}

	return tokenRequirementResponse, nil
}

func (p *DefaultPermissionChecker) checkPermissions(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool,
	getOwnedERC721Tokens ownedERC721TokensGetter, getBalancesByChain balancesByChainGetter) (*CheckPermissionsResponse, error) {

	response := &CheckPermissionsResponse{
		Satisfied:         false,
		Permissions:       make(map[string]*PermissionTokenCriteriaResult),
		ValidCombinations: make([]*AccountChainIDsCombination, 0),
	}

	if permissionsParsedData == nil {
		response.Satisfied = true
		return response, nil
	}

	erc721TokenRequirements := permissionsParsedData.Erc721TokenRequirements

	erc20ChainIDsMap := permissionsParsedData.Erc20ChainIDsMap
	erc721ChainIDsMap := permissionsParsedData.Erc721ChainIDsMap

	erc20TokenAddresses := permissionsParsedData.Erc20TokenAddresses

	accounts := make([]gethcommon.Address, 0)

	// TODO: move outside in order not to convert it
	for _, accountAndChainIDs := range accountsAndChainIDs {
		accounts = append(accounts, accountAndChainIDs.Address)
	}

	chainIDsForERC20 := calculateChainIDsSet(accountsAndChainIDs, erc20ChainIDsMap)
	chainIDsForERC721 := calculateChainIDsSet(accountsAndChainIDs, erc721ChainIDsMap)

	// if there are no chain IDs that match token criteria chain IDs
	// we aren't able to check balances on selected networks
	if len(erc20ChainIDsMap) > 0 && len(chainIDsForERC20) == 0 {
		response.NetworksNotSupported = true
		return response, nil
	}

	ownedERC20TokenBalances := make(map[uint64]map[gethcommon.Address]map[gethcommon.Address]*hexutil.Big, 0)
	if len(chainIDsForERC20) > 0 {
		// this only returns balances for the networks we're actually interested in
		balances, err := getBalancesByChain(context.Background(), accounts, erc20TokenAddresses, chainIDsForERC20)
		if err != nil {
			return nil, err
		}
		ownedERC20TokenBalances = balances
	}

	ownedERC721Tokens := make(CollectiblesByChain)
	if len(chainIDsForERC721) > 0 {
		collectibles, err := getOwnedERC721Tokens(accounts, erc721TokenRequirements, chainIDsForERC721)
		if err != nil {
			return nil, err
		}
		ownedERC721Tokens = collectibles
	}

	accountsChainIDsCombinations := make(map[gethcommon.Address]map[uint64]bool)

	for _, tokenPermission := range permissionsParsedData.Permissions {
		permissionRequirementsMet := true
		response.Permissions[tokenPermission.Id] = &PermissionTokenCriteriaResult{Role: tokenPermission.Type}

		// There can be multiple token requirements per permission.
		// If only one is not met, the entire permission is marked
		// as not fulfilled
		for _, tokenRequirement := range tokenPermission.TokenCriteria {
			tokenRequirementResponse, err := p.checkTokenRequirement(tokenRequirement, accounts, ownedERC20TokenBalances, ownedERC721Tokens, accountsChainIDsCombinations)
			if err != nil {
				p.logger.Error("failed to check token requirement", zap.Error(err))
			}

			if !tokenRequirementResponse.Satisfied {
				permissionRequirementsMet = false
			}

			response.Permissions[tokenPermission.Id].TokenRequirements = append(response.Permissions[tokenPermission.Id].TokenRequirements, tokenRequirementResponse)
			response.Permissions[tokenPermission.Id].Criteria = append(response.Permissions[tokenPermission.Id].Criteria, tokenRequirementResponse.Satisfied)
		}
		response.Permissions[tokenPermission.Id].ID = tokenPermission.Id

		// multiple permissions are treated as logical OR, meaning
		// if only one of them is fulfilled, the user gets permission
		// to join and we can stop early
		if shortcircuit && permissionRequirementsMet {
			break
		}
	}

	// attach valid account and chainID combinations to response
	for account, chainIDs := range accountsChainIDsCombinations {
		combination := &AccountChainIDsCombination{
			Address: account,
		}
		for chainID := range chainIDs {
			combination.ChainIDs = append(combination.ChainIDs, chainID)
		}
		response.ValidCombinations = append(response.ValidCombinations, combination)
	}

	response.calculateSatisfied()

	return response, nil
}

type balancesByOwnerAndContractAddressGetter = func(ctx context.Context, chainID walletcommon.ChainID, ownerAddress gethcommon.Address, contractAddresses []gethcommon.Address) (map[gethcommon.Address][]thirdparty.TokenBalance, error)

func (p *DefaultPermissionChecker) handlePermissionsCheck(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool,
	getBalancesByOwnerAndContractAddress balancesByOwnerAndContractAddressGetter,
	getBalancesByChain balancesByChainGetter) (*CheckPermissionsResponse, error) {

	var getOwnedERC721Tokens ownedERC721TokensGetter = func(walletAddresses []gethcommon.Address, tokenRequirements map[uint64]map[string]*protobuf.TokenCriteria, chainIDs []uint64) (CollectiblesByChain, error) {
		return p.getOwnedERC721Tokens(walletAddresses, tokenRequirements, chainIDs, getBalancesByOwnerAndContractAddress)
	}

	return p.checkPermissions(permissionsParsedData, accountsAndChainIDs, shortcircuit, getOwnedERC721Tokens, getBalancesByChain)
}

func (p *DefaultPermissionChecker) CheckCachedPermissions(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool) (*CheckPermissionsResponse, error) {
	return p.handlePermissionsCheck(permissionsParsedData, accountsAndChainIDs, shortcircuit, p.collectiblesManager.FetchCachedBalancesByOwnerAndContractAddress, p.tokenManager.GetCachedBalancesByChain)
}

// CheckPermissions will retrieve balances and check whether the user has
// permission to join the community, if shortcircuit is true, it will stop as soon
// as we know the answer
func (p *DefaultPermissionChecker) CheckPermissions(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool) (*CheckPermissionsResponse, error) {
	return p.handlePermissionsCheck(permissionsParsedData, accountsAndChainIDs, shortcircuit, p.collectiblesManager.FetchBalancesByOwnerAndContractAddress, p.tokenManager.GetBalancesByChain)
}

type CollectiblesOwners = map[walletcommon.ChainID]map[gethcommon.Address]*thirdparty.CollectibleContractOwnership

// Same as CheckPermissions but relies on already provided collectibles owners
func (p *DefaultPermissionChecker) CheckPermissionsWithPreFetchedData(permissionsParsedData *PreParsedCommunityPermissionsData, accountsAndChainIDs []*AccountChainIDsCombination, shortcircuit bool, collectiblesOwners CollectiblesOwners) (*CheckPermissionsResponse, error) {
	var getCollectiblesBalances collectiblesBalancesGetter = func(ctx context.Context, chainID walletcommon.ChainID, ownerAddress gethcommon.Address, contractAddresses []gethcommon.Address) (thirdparty.TokenBalancesPerContractAddress, error) {
		ret := make(thirdparty.TokenBalancesPerContractAddress)

		collectiblesByChain, ok := collectiblesOwners[chainID]
		if !ok {
			return nil, errors.New("no data available for chainID")
		}

		for _, contractAddress := range contractAddresses {
			ownership, ok := collectiblesByChain[contractAddress]
			if !ok {
				return nil, errors.New("no data available for collectible")
			}

			for _, nftOwner := range ownership.Owners {
				if nftOwner.OwnerAddress == ownerAddress {
					ret[contractAddress] = nftOwner.TokenBalances
					break
				}
			}
		}

		return ret, nil
	}

	var getOwnedERC721Tokens ownedERC721TokensGetter = func(walletAddresses []gethcommon.Address, tokenRequirements map[uint64]map[string]*protobuf.TokenCriteria, chainIDs []uint64) (CollectiblesByChain, error) {
		return p.getOwnedERC721Tokens(walletAddresses, tokenRequirements, chainIDs, getCollectiblesBalances)
	}

	return p.checkPermissions(permissionsParsedData, accountsAndChainIDs, shortcircuit, getOwnedERC721Tokens, p.tokenManager.GetBalancesByChain)
}

func preParsedPermissionsData(permissions []*CommunityTokenPermission) *PreParsedPermissionsData {
	erc20TokenRequirements, erc721TokenRequirements, _ := ExtractTokenCriteria(permissions)

	erc20ChainIDsMap := make(map[uint64]bool)
	erc721ChainIDsMap := make(map[uint64]bool)

	erc20TokenAddresses := make([]gethcommon.Address, 0)

	// figure out chain IDs we're interested in
	for chainID, tokens := range erc20TokenRequirements {
		erc20ChainIDsMap[chainID] = true
		for contractAddress := range tokens {
			erc20TokenAddresses = append(erc20TokenAddresses, gethcommon.HexToAddress(contractAddress))
		}
	}

	for chainID := range erc721TokenRequirements {
		erc721ChainIDsMap[chainID] = true
	}

	return &PreParsedPermissionsData{
		Erc721TokenRequirements: erc721TokenRequirements,
		Erc20TokenAddresses:     erc20TokenAddresses,
		Erc20ChainIDsMap:        erc20ChainIDsMap,
		Erc721ChainIDsMap:       erc721ChainIDsMap,
	}
}

func preParsedCommunityPermissionsData(permissions []*CommunityTokenPermission) *PreParsedCommunityPermissionsData {
	if len(permissions) == 0 {
		return nil
	}

	return &PreParsedCommunityPermissionsData{
		Permissions:              permissions,
		PreParsedPermissionsData: preParsedPermissionsData(permissions),
	}
}

func PreParsePermissionsData(permissions map[string]*CommunityTokenPermission) (map[protobuf.CommunityTokenPermission_Type]*PreParsedCommunityPermissionsData, map[string]*PreParsedCommunityPermissionsData) {
	becomeMemberPermissions := TokenPermissionsByType(permissions, protobuf.CommunityTokenPermission_BECOME_MEMBER)
	becomeAdminPermissions := TokenPermissionsByType(permissions, protobuf.CommunityTokenPermission_BECOME_ADMIN)
	becomeTokenMasterPermissions := TokenPermissionsByType(permissions, protobuf.CommunityTokenPermission_BECOME_TOKEN_MASTER)

	viewOnlyPermissions := TokenPermissionsByType(permissions, protobuf.CommunityTokenPermission_CAN_VIEW_CHANNEL)
	viewAndPostPermissions := TokenPermissionsByType(permissions, protobuf.CommunityTokenPermission_CAN_VIEW_AND_POST_CHANNEL)
	channelPermissions := append(viewAndPostPermissions, viewOnlyPermissions...)

	communityPermissionsPreParsedData := make(map[protobuf.CommunityTokenPermission_Type]*PreParsedCommunityPermissionsData)
	communityPermissionsPreParsedData[protobuf.CommunityTokenPermission_BECOME_MEMBER] = preParsedCommunityPermissionsData(becomeMemberPermissions)
	communityPermissionsPreParsedData[protobuf.CommunityTokenPermission_BECOME_ADMIN] = preParsedCommunityPermissionsData(becomeAdminPermissions)
	communityPermissionsPreParsedData[protobuf.CommunityTokenPermission_BECOME_TOKEN_MASTER] = preParsedCommunityPermissionsData(becomeTokenMasterPermissions)

	channelPermissionsPreParsedData := make(map[string]*PreParsedCommunityPermissionsData)
	for _, channelPermission := range channelPermissions {
		channelPermissionsPreParsedData[channelPermission.Id] = preParsedCommunityPermissionsData([]*CommunityTokenPermission{channelPermission})
	}

	return communityPermissionsPreParsedData, channelPermissionsPreParsedData
}

func CollectibleAddressesFromPreParsedPermissionsData(communityPermissions map[protobuf.CommunityTokenPermission_Type]*PreParsedCommunityPermissionsData, channelPermissions map[string]*PreParsedCommunityPermissionsData) map[walletcommon.ChainID]map[gethcommon.Address]struct{} {
	ret := make(map[walletcommon.ChainID]map[gethcommon.Address]struct{})

	allPermissionsData := []*PreParsedCommunityPermissionsData{}
	for _, permissionsData := range communityPermissions {
		if permissionsData != nil {
			allPermissionsData = append(allPermissionsData, permissionsData)
		}
	}
	for _, permissionsData := range channelPermissions {
		if permissionsData != nil {
			allPermissionsData = append(allPermissionsData, permissionsData)
		}
	}

	for _, data := range allPermissionsData {
		for chainID, contractAddresses := range data.Erc721TokenRequirements {
			if ret[walletcommon.ChainID(chainID)] == nil {
				ret[walletcommon.ChainID(chainID)] = make(map[gethcommon.Address]struct{})
			}

			for contractAddress := range contractAddresses {
				ret[walletcommon.ChainID(chainID)][gethcommon.HexToAddress(contractAddress)] = struct{}{}
			}
		}
	}

	return ret
}
