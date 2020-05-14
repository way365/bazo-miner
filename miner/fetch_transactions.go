package miner

import (
	"errors"
	"github.com/julwil/bazo-miner/protocol"
	"time"
)

// Fetch all txs for a given block and place them into the provided slices.
func fetchTransactions(
	initialSetup bool,
	block *protocol.Block,
	accTxSlice *[]*protocol.AccTx,
	fundsTxSlice *[]*protocol.FundsTx,
	configTxSlice *[]*protocol.ConfigTx,
	stakeTxSlice *[]*protocol.StakeTx,
	aggTxSlice *[]*protocol.AggTx,
	aggregatedFundsTxSlice *[]*protocol.FundsTx,
	deleteTxSlice *[]*protocol.DeleteTx,
) error {

	//We fetch tx data for each type in parallel -> performance boost.
	nrOfChannels := 6
	errChan := make(chan error, nrOfChannels)
	aggregatedFundsChan := make(chan []*protocol.FundsTx, 10000)

	go fetchAccTxData(block, *accTxSlice, initialSetup, errChan)
	go fetchFundsTxData(block, *fundsTxSlice, initialSetup, errChan)
	go fetchConfigTxData(block, *configTxSlice, initialSetup, errChan)
	go fetchStakeTxData(block, *stakeTxSlice, initialSetup, errChan)
	go fetchDeleteTxData(block, *deleteTxSlice, initialSetup, errChan)
	go fetchAggTxData(block, *aggTxSlice, initialSetup, errChan, aggregatedFundsChan)

	//Wait for all goroutines to finish.
	for cnt := 0; cnt < nrOfChannels; cnt++ {
		err := <-errChan
		if err != nil {
			return err
		}
	}

	if len(*aggTxSlice) > 0 {
		logger.Printf("-- Fetch AggTxData - Start")
		select {
		case *aggregatedFundsTxSlice = <-aggregatedFundsChan:
		case <-time.After(10 * time.Minute):
			return errors.New("Fetching FundsTx aggregated in AggTx failed.")
		}
		logger.Printf("-- Fetch AggTxData - End")
	}

	return nil
}
