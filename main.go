package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type txInfo struct {
	TxNum         int
	TxHash        string
	YoloGasUsed   int
	YoloSteps     int
	YoloError     bool
	Yolo2xGasUsed int
	Yolo2xSteps   int
	Yolo2xError   bool
	MainGasUsed   int
	MainSteps     int
	MainError     bool

	BlockHash string
}

type Stats struct {
	numBlocks   int
	numTxs      int
	emptyBlocks int

	unaffected  int
	mainError   int
	salvageable int
	broken      int

	gasUsedMain   int
	gasUsedYolo   int
	gasUsedYolo2x int

	maxDiff     int
	maxDiffBase int
	maxDiffYolo int
}

func (stats *Stats) print() {
	fmt.Printf("Number of blocks: `%v`\n", stats.numBlocks)
	fmt.Printf(" - number of empty (ignored) blocks: `%v`\n", stats.emptyBlocks)
	fmt.Printf("Number of transactions: `%v`\n", stats.numTxs)
	fmt.Println()
	fmt.Printf("Number of unaffected transactions: `%v`\n", stats.unaffected)
	fmt.Printf("- broken on mainnet already: `%v`\n", stats.mainError)
	fmt.Printf("Number of salvageable transactions: `%v`\n", stats.salvageable)
	fmt.Printf("Number of broken transactions: `%v`\n", stats.broken)

	y := stats.gasUsedYolo + stats.gasUsedYolo2x
	m := stats.gasUsedMain
	percent := float64(100*100*(y-m)/m) / 100
	fmt.Println()
	fmt.Printf("Gas usage for mainnet vs yolo: `%d` vs `%d`\n", m, y)
	fmt.Printf("The gas usage with yolo rules is `%.02f %%`\n", percent)
	fmt.Println()
	fmt.Printf("Largest EIP-2929 gas difference %d (from `%d` to `%d`).\n",
		stats.maxDiff, stats.maxDiffBase, stats.maxDiffYolo)

}

func parseFiles() ([]txInfo, *Stats, error) {
	var data []txInfo
	finfo, err := ioutil.ReadDir("./rawdata/")
	if err != nil {
		return nil, nil, err
	}
	stats := &Stats{}
	for _, info := range finfo {
		if !strings.HasPrefix(info.Name(), "block_") {
			continue
		}
		stats.numBlocks++
		var blockdata []txInfo
		byts, err := ioutil.ReadFile(fmt.Sprintf("./rawdata/%v", info.Name()))
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(byts, &blockdata)
		if err != nil {
			return nil, nil, err
		}
		stats.numTxs += len(blockdata)
		if len(blockdata) == 0 {
			stats.emptyBlocks++
		}
		for _, tx := range blockdata {
			tx.BlockHash = info.Name()
			data = append(data, tx)
		}
	}
	return data, stats, nil
}

func analyseTransactions(txs []txInfo, stats *Stats, removeFP bool) error {
	for _, tx := range txs {
		if tx.MainError {
			stats.mainError++
			stats.unaffected++
			continue
		}
		if !tx.YoloError && tx.MainSteps == tx.YoloSteps {
			stats.gasUsedMain += tx.MainGasUsed
			stats.gasUsedYolo += tx.YoloGasUsed
			stats.unaffected++
			if diff := (tx.Yolo2xGasUsed - tx.MainGasUsed); diff > stats.maxDiff {
				stats.maxDiff = diff
				stats.maxDiffBase = tx.MainGasUsed
				stats.maxDiffYolo = tx.Yolo2xGasUsed
			}
			continue
		}
		if !tx.Yolo2xError && tx.MainSteps == tx.Yolo2xSteps {
			stats.gasUsedMain += tx.MainGasUsed
			stats.gasUsedYolo += tx.Yolo2xGasUsed
			stats.salvageable++
			if diff := (tx.Yolo2xGasUsed - tx.MainGasUsed); diff > stats.maxDiff {
				stats.maxDiff = diff
				stats.maxDiffBase = tx.MainGasUsed
				stats.maxDiffYolo = tx.Yolo2xGasUsed
			}
			continue
		}
		if removeFP {

			if !tx.YoloError && tx.MainSteps <= tx.YoloSteps {
				stats.gasUsedMain += tx.MainGasUsed
				stats.gasUsedYolo += tx.YoloGasUsed
				stats.unaffected++
				if diff := (tx.Yolo2xGasUsed - tx.MainGasUsed); diff > stats.maxDiff {
					stats.maxDiff = diff
					stats.maxDiffBase = tx.MainGasUsed
					stats.maxDiffYolo = tx.Yolo2xGasUsed
				}
				continue
			}
			if !tx.Yolo2xError && tx.MainSteps <= tx.Yolo2xSteps {
				stats.gasUsedMain += tx.MainGasUsed
				stats.gasUsedYolo += tx.Yolo2xGasUsed
				stats.salvageable++
				if diff := (tx.Yolo2xGasUsed - tx.MainGasUsed); diff > stats.maxDiff {
					stats.maxDiff = diff
					stats.maxDiffBase = tx.MainGasUsed
					stats.maxDiffYolo = tx.Yolo2xGasUsed
				}
				continue
			}
		}

		fmt.Printf("tx txbase-steps: %d, tx2929-steps: %d, tx2929b-steps: %d, tx2929b-Error: %v, block:%v,  txHash: %v\n",
			tx.MainSteps, tx.YoloSteps, tx.Yolo2xSteps, tx.Yolo2xError, tx.BlockHash, tx.TxHash)
		stats.broken++
	}
	return nil
}

// Example of contract which didn't work with 2x gas: https://etherscan.io/address/0x2b591e99afe9f32eaa6214f7b7629768c40eeb39#code
// tx mainsteps: 18655, yolosteps: 9053, yolo2xsteps: 18156, yolo2Error: true, block:block_11354984-0xccc7e971-analysis-223546762,  txHash: 0x4930308d13b9ac5b49214971412f115339e7e88480b420bb04b42631d49ad9a4
// Does a lot of SLOAD on `stakeEnd`.

func main() {
	data, stats, err := parseFiles()
	if err != nil {
		fmt.Fprint(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	analyseTransactions(data, stats, true)
	stats.print()
}
