// Package storage defines storage interfaces.
package storage

import (
	"context"

	"github.com/jackc/pgx/v4"
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// QueryBatch represents a batch of queries to be executed atomically.
type QueryBatch = pgx.Batch

// QueryResults represents the results from a read query.
type QueryResults = pgx.Rows

// QueryResult represents the result from a read query.
type QueryResult = pgx.Row

// ConsensusSourceStorage defines an interface for retrieving raw block data
// from the consensus layer.
type ConsensusSourceStorage interface {
	// BlockData gets block data at the specified height. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	BlockData(ctx context.Context, height int64) (*ConsensusBlockData, error)

	// BeaconData gets beacon data at the specified height. This includes
	// the epoch number at that height, as well as the beacon state.
	BeaconData(ctx context.Context, height int64) (*BeaconData, error)

	// RegistryData gets registry data at the specified height. This includes
	// all registered entities and their controlled nodes and statuses.
	RegistryData(ctx context.Context, height int64) (*RegistryData, error)

	// StakingData gets staking data at the specified height. This includes
	// staking backend events to be applied to indexed state.
	StakingData(ctx context.Context, height int64) (*StakingData, error)

	// SchedulerData gets scheduler data at the specified height. This
	// includes all validators and runtime committees.
	SchedulerData(ctx context.Context, height int64) (*SchedulerData, error)

	// GovernanceData gets governance data at the specified height. This
	// includes all proposals, their respective statuses and voting responses.
	GovernanceData(ctx context.Context, height int64) (*GovernanceData, error)

	// TODO: Extend this interface to include a GetRoothashData to pull
	// runtime blocks. This is only relevant when we begin to build runtime
	// analyzers.

	// Name returns the name of the source storage.
	Name() string
}

// ConsensusBlockData represents data for a consensus block at a given height.
type ConsensusBlockData struct {
	Height int64

	BlockHeader  *consensus.Block
	Epoch        beacon.EpochTime
	Transactions []*transaction.SignedTransaction
	Results      []*results.Result
}

// BeaconData represents data for the random beacon at a given height.
type BeaconData struct {
	Height int64

	Epoch  beacon.EpochTime
	Beacon []byte
}

// RegistryData represents data for the node registry at a given height.
//
// Note: The registry backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type RegistryData struct {
	Height int64

	RuntimeEvents      []*registry.RuntimeEvent
	EntityEvents       []*registry.EntityEvent
	NodeEvents         []*registry.NodeEvent
	NodeUnfrozenEvents []*registry.NodeUnfrozenEvent

	RuntimeSuspensions   []string
	RuntimeUnsuspensions []string
}

// StakingData represents data for accounts at a given height.
//
// Note: The staking backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type StakingData struct {
	Height int64
	Epoch  beacon.EpochTime

	Transfers        []*staking.TransferEvent
	Burns            []*staking.BurnEvent
	Escrows          []*staking.EscrowEvent
	AllowanceChanges []*staking.AllowanceChangeEvent
}

// SchedulerData represents data for elected committees and validators at a given height.
type SchedulerData struct {
	Height int64

	Validators []*scheduler.Validator
	Committees map[common.Namespace][]*scheduler.Committee
}

// GovernanceData represents governance data for proposals at a given height.
//
// Note: The governance backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at a specific height.
type GovernanceData struct {
	Height int64

	ProposalSubmissions   []*governance.Proposal
	ProposalExecutions    []*governance.ProposalExecutedEvent
	ProposalFinalizations []*governance.Proposal
	Votes                 []*governance.VoteEvent
}

// RuntimeSourceStorage defines an interface for retrieving raw block data
// from the runtime layer.
type RuntimeSourceStorage interface {
	// BlockData gets block data in the specified round. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	BlockData(ctx context.Context, round uint64) (*RuntimeBlockData, error)

	// CoreData gets data in the specified round emitted by the `core` module.
	CoreData(ctx context.Context, round uint64) (*CoreData, error)

	// AccountsData gets data in the specified round emitted by the `accounts` module.
	AccountsData(ctx context.Context, round uint64) (*AccountsData, error)

	// ConsensusAccountsData gets data in the specified round emitted by the `consensusaccounts` module.
	ConsensusAccountsData(ctx context.Context, round uint64) (*ConsensusAccountsData, error)

	// Name returns the name of the source storage.
	Name() string
}

// RuntimeBlockData represents data for a runtime block during a given round.
type RuntimeBlockData struct {
	Round uint64

	BlockHeader             *block.Block
	TransactionsWithResults []*client.TransactionWithResults
}

// TransactionWithResults contains a verified transaction, and the results of
// executing that transactions.
type TransactionWithResults struct {
	Round uint64

	Tx     *types.Transaction
	Result types.CallResult
	Events []*types.Event
}

// CoreData represents data from the `core` module for a runtime.
type CoreData struct {
	Round uint64

	GasUsed []*core.GasUsedEvent
}

// AccountsData represents data from the `accounts` module for a runtime.
type AccountsData struct {
	Round uint64

	Transfers []*accounts.TransferEvent
	Burns     []*accounts.BurnEvent
	Mints     []*accounts.MintEvent
}

// ConsensusAccounts represents data from the `consensusaccounts` module for a runtime.
type ConsensusAccountsData struct {
	Round uint64

	Deposits  []*consensusaccounts.DepositEvent
	Withdraws []*consensusaccounts.WithdrawEvent
}

// TargetStorage defines an interface for reading and writing
// processed block data.
type TargetStorage interface {
	// SendBatch sends a batch of queries to be applied to target storage.
	SendBatch(ctx context.Context, batch *QueryBatch) error

	// Query submits a query to fetch data from target storage.
	Query(ctx context.Context, sql string, args ...interface{}) (QueryResults, error)

	// QueryRow submits a query to fetch a single row of data from target storage.
	QueryRow(ctx context.Context, sql string, args ...interface{}) QueryResult

	// Shutdown shuts down the target storage client.
	Shutdown()

	// Name returns the name of the target storage.
	Name() string
}
