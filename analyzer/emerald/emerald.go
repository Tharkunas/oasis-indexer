package emerald

import (
	"context"
	"fmt"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jackc/pgx/v4"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"golang.org/x/sync/errgroup"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/modules"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	emerald = analyzer.RuntimeEmerald

	emeraldMainDamaskName = "emerald_main_damask"
)

// Main is the main Analyzer for the Emerald Runtime.
type Main struct {
	cfg     analyzer.RuntimeConfig
	qf      analyzer.QueryFactory
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics

	moduleHandlers []modules.ModuleHandler
}

// NewMain returns a new main analyzer for the Emerald Runtime.
func NewMain(cfg *config.AnalyzerConfig, target storage.TargetStorage, logger *log.Logger) (*Main, error) {
	ctx := context.Background()

	// Initialize source storage.
	networkCfg := oasisConfig.Network{
		ChainContext: cfg.ChainContext,
		RPC:          cfg.RPC,
	}
	factory, err := oasis.NewClientFactory(ctx, &networkCfg)
	if err != nil {
		logger.Error("error creating client factory",
			"err", err,
		)
		return nil, err
	}

	network, err := analyzer.FromChainContext(cfg.ChainContext)
	if err != nil {
		return nil, err
	}

	id, err := emerald.ID(network)
	if err != nil {
		return nil, err
	}

	client, err := factory.Runtime(id)
	if err != nil {
		logger.Error("error creating runtime client",
			"err", err,
		)
		return nil, err
	}
	roundRange := analyzer.RoundRange{
		From: uint64(cfg.From),
		To:   uint64(cfg.To),
	}
	ac := analyzer.RuntimeConfig{
		Range:  roundRange,
		Source: client,
	}

	qf := analyzer.NewQueryFactory(strcase.ToSnake(cfg.ChainID), emerald.String())

	return &Main{
		cfg:     ac,
		qf:      qf,
		target:  target,
		logger:  logger,
		metrics: metrics.NewDefaultDatabaseMetrics(emeraldMainDamaskName),

		// module handlers
		moduleHandlers: []modules.ModuleHandler{
			modules.NewCoreHandler(client, &qf, logger),
			modules.NewAccountsHandler(client, &qf, logger),
			modules.NewConsensusAccountsHandler(client, &qf, logger),
		},
	}, nil
}

func (m *Main) Start() {
	ctx := context.Background()

	// Get round to be indexed.
	var round uint64

	latest, err := m.latestRound(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			m.logger.Error("last round not found",
				"err", err,
			)
			return
		}
		m.logger.Debug("setting round using range config")
		round = m.cfg.Range.From
	} else {
		m.logger.Debug("setting round using latest round")
		round = latest + 1
	}

	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second,
		// ^cap the timeout at the expected
		// emerald round time
	)
	if err != nil {
		m.logger.Error("error configuring indexer backoff policy",
			"err", err,
		)
		return
	}
	for m.cfg.Range.To == 0 || round <= m.cfg.Range.To {
		if err := m.processRound(ctx, round); err != nil {
			if err == analyzer.ErrOutOfRange {
				m.logger.Info("no data source available for this round",
					"round", round,
				)
				return
			}

			m.logger.Error("error processing round",
				"err", err,
			)
			backoff.Wait()
			continue
		}

		backoff.Reset()
		round++
	}
}

// Name returns the name of the Main.
func (m *Main) Name() string {
	return emeraldMainDamaskName
}

// latestRound returns the latest round processed by the consensus analyzer.
func (m *Main) latestRound(ctx context.Context) (uint64, error) {
	var latest uint64
	if err := m.target.QueryRow(
		ctx,
		m.qf.LatestBlockQuery(),
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting round.
		emeraldMainDamaskName,
	).Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// processRound processes the provided round, retrieving all required information
// from source storage and committing an atomically-executed batch of queries
// to target storage.
func (m *Main) processRound(ctx context.Context, round uint64) error {
	m.logger.Info("processing round",
		"round", round,
	)

	group, groupCtx := errgroup.WithContext(ctx)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	group.Go(func() error {
		if err := m.prepareBlockData(ctx, round, batch); err != nil {
			return err
		}
		return nil
	})

	type prepareFunc = func(context.Context, uint64, *storage.QueryBatch) error
	for _, h := range m.moduleHandlers {
		func(f prepareFunc) {
			group.Go(func() error {
				if err := f(groupCtx, round, batch); err != nil {
					return err
				}
				return nil
			})
		}(h.PrepareData)
	}

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(
			m.qf.IndexingProgressQuery(),
			round,
			emeraldMainDamaskName,
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	opName := fmt.Sprintf("process_round_%s", emerald.String())
	timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseCounter(m.target.Name(), opName, "success").Inc()
	return nil
}

// prepareBlockData adds block data queries to the batch.
func (m *Main) prepareBlockData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := m.cfg.Source.BlockData(ctx, round)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RuntimeBlockData) error{
		m.queueBlockInserts,
		m.queueTransactionInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueBlockInserts(batch *storage.QueryBatch, data *storage.RuntimeBlockData) error {
	batch.Queue(
		m.qf.RuntimeBlockInsertQuery(),
		data.Round,
		data.BlockHeader.Header.Version,
		data.BlockHeader.Header.Timestamp,
		data.BlockHeader.Header.EncodedHash().Hex(),
		data.BlockHeader.Header.PreviousHash.Hex(),
		data.BlockHeader.Header.IORoot.Hex(),
		data.BlockHeader.Header.StateRoot.Hex(),
		data.BlockHeader.Header.MessagesHash.Hex(),
		data.BlockHeader.Header.InMessagesHash.Hex(),
	)
	return nil
}

func (m *Main) queueTransactionInserts(batch *storage.QueryBatch, data *storage.RuntimeBlockData) error {
	// TODO(ennsharma): Process Emerald transaction data in accordance with
	// https://github.com/oasisprotocol/emerald-web3-gateway/blob/main/indexer/utils.go#L225
	return nil
}
