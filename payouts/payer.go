package payouts

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"
  "strings"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/hostup/open-zano-pool/rpc"
	"github.com/hostup/open-zano-pool/storage"
	"github.com/hostup/open-zano-pool/util"
)

const txCheckInterval = 5 * time.Second

type PayoutsConfig struct {
	Enabled      bool   `json:"enabled"`
	RequirePeers uint64 `json:"requirePeers"`
	Interval     string `json:"interval"`
	Daemon       string `json:"daemon"`
  Wallet       string `json:"wallet"`
	Timeout      string `json:"timeout"`
	Address      string `json:"address"`
	Gas          string `json:"gas"`
	GasPrice     string `json:"gasPrice"`
	AutoGas      bool   `json:"autoGas"`
	KeepNwFees   bool   `json:"keepNwFees"`
	TxGas        string `json:"nwTxGas"`
	TxGasPrice   string `json:"nwTxGasPrice"`
	// In Shannon
	Threshold int64 `json:"threshold"`
	BgSave    bool  `json:"bgsave"`
}

func (self PayoutsConfig) GasHex() string {
	x := util.String2Big(self.Gas)
	return hexutil.EncodeBig(x)
}

func (self PayoutsConfig) GasPriceHex() string {
	x := util.String2Big(self.GasPrice)
	return hexutil.EncodeBig(x)
}

type PayoutsProcessor struct {
	config     *PayoutsConfig
	backend    *storage.RedisClient
	rpc_daemon *rpc.RPCClient
  rpc_wallet *rpc.RPCClient
	halt       bool
	lastFail   error
}

func NewPayoutsProcessor(cfg *PayoutsConfig, backend *storage.RedisClient) *PayoutsProcessor {
	u := &PayoutsProcessor{config: cfg, backend: backend}
	u.rpc_daemon = rpc.NewRPCClient("PayoutsDaemon", cfg.Daemon, cfg.Timeout)
  u.rpc_wallet = rpc.NewRPCClient("PayoutsWallet", cfg.Wallet, cfg.Timeout)
  return u
}

func (u *PayoutsProcessor) Start() {
	log.Println("Starting payouts")

	if u.mustResolvePayout() {
		log.Println("Running with env RESOLVE_PAYOUT=1, now trying to resolve locked payouts")
		u.resolvePayouts()
		log.Println("Now you have to restart payouts module with RESOLVE_PAYOUT=0 for normal run")
		return
	}

	intv := util.MustParseDuration(u.config.Interval)
	timer := time.NewTimer(intv)
	log.Printf("Set payouts interval to %v", intv)

	payments := u.backend.GetPendingPayments()
	if len(payments) > 0 {
		log.Printf("Previous payout failed, you have to resolve it. List of failed payments:\n %v",
			formatPendingPayments(payments))
		return
	}

	locked, err := u.backend.IsPayoutsLocked()
	if err != nil {
		log.Println("Unable to start payouts:", err)
		return
	}
	if locked {
		log.Println("Unable to start payouts because they are locked")
		return
	}

	// Immediately process payouts after start
	u.process()
	timer.Reset(intv)

	go func() {
		for {
			select {
			case <-timer.C:
				u.process()
				timer.Reset(intv)
			}
		}
	}()
}

func (u *PayoutsProcessor) process() {
	baseFee := uint64(10000000000)
  if u.halt {
		log.Println("Payments suspended due to last critical error:", u.lastFail)
		return
	}
	mustPay := 0
	minersPaid := 0
	totalAmount := big.NewInt(0)
	payees, err := u.backend.GetPayees()
	if err != nil {
		log.Println("Error while retrieving payees from backend:", err)
		return
	}

	xferdests := []rpc.TransferDestination{}
  integdests := make(map[string]rpc.TransferDestination)
  tempAmounts := make(map[string]int64)
  tempAmountsIntegrated := make(map[string]int64)
  totalPayoutInWei := new(big.Int).SetInt64(0)
  poolBalance, err := u.rpc_wallet.GetBalance()
  if err != nil {
      u.halt = true
      u.lastFail = err
      return
  }

	for _, login := range payees {
		amount, _ := u.backend.GetBalance(login)
		amountInShannon := big.NewInt(amount)

		ptresh, _ := u.backend.GetThreshold(login)
		if ptresh <= 10 {
			ptresh = u.config.Threshold
		}

		// Shannon^2 = Wei
		amountInWei := new(big.Int).Mul(amountInShannon, util.Shannon)

		if !u.reachedThreshold(amountInShannon, ptresh) {
			continue
		}
		mustPay++

		// Require active peers before processing
		if !u.checkPeers() {
			break
		}

		//Calculate the Gas Price in Wei and Computer the Transaction Charges
		//Since pool honour only mining to wallet and not to contract, Deduct value equal to gas*21000 - Standard cost price

		TxCharges := big.NewInt(0)

		if u.config.KeepNwFees {

			TxCharges.Mul(util.String2Big(u.config.TxGasPrice), util.String2Big(u.config.TxGas))

			//Deduct the Calulated Transaction Charges
			amountInWei.Sub(amountInWei, TxCharges)

		}

		value := amountInWei.Uint64()
    totalPayoutInWei.Add(totalPayoutInWei, amountInWei)
    var xferdest rpc.TransferDestination
    xferdest.Amount = value
    xferdest.Address = login
		if strings.HasPrefix(login, "iZ") || strings.HasPrefix(login, "aiZX") {
      tempAmountsIntegrated[login] = amount
      // go ahead and remove the tx fee here
      xferdest.Amount = xferdest.Amount - baseFee
      integdests[login] = xferdest
    } else {
      tempAmounts[login] = amount
      xferdests = append(xferdests, xferdest)
    }
	}

	if mustPay > 0 {
    if poolBalance.Cmp(totalPayoutInWei) < 0 {
      err := fmt.Errorf("Not enough balance for payment, need %s pZano, pool has %s pZano",
      totalPayoutInWei.String(), poolBalance.String())
      u.halt = true
      u.lastFail = err
      return
    }
		// split the tx fee evenly over all bulk-tx recipients
    // those first in line are a little unlucky
		if len(tempAmounts) > 0 {
      nPayees := uint64(len(xferdests))
      feePerPayee := baseFee / nPayees
      feeRemainder := int64(baseFee % nPayees)
      for k := 0; k < len(xferdests); k++ {
        extra := uint64(0)
        if feeRemainder > 0 {
          extra = 1
          feeRemainder = feeRemainder - 1
        }
        xferdests[k].Amount = xferdests[k].Amount - feePerPayee - extra
      }

      txHash, err := u.rpc_wallet.SendTransaction(xferdests, baseFee, 0)
      if err != nil {
			log.Printf("Failed to send payment to %v. Check outgoing tx for %s in block explorer and docs/PAYOUTS.md",
				xferdests, txHash)
			u.halt = true
			u.lastFail = err
			return
		}
		for login, amount := range tempAmounts {
			// Lock payments for current payout
			err = u.backend.LockPayouts(login, amount)
			if err != nil {
				log.Printf("Failed to lock payment for %s: %v", login, err)
				u.halt = true
				u.lastFail = err
				break
			}
			log.Printf("Locked payment for %s, %v Shannon", login, amount)

			// Debit miner's balance and update stats
			err = u.backend.UpdateBalance(login, amount)
			if err != nil {
				log.Printf("Failed to update balance for %s, %v Shannon: %v", login, amount, err)
				u.halt = true
				u.lastFail = err
				break
			}

			// Log transaction hash
			err = u.backend.WritePayment(login, txHash, amount)
			if err != nil {
				log.Printf("Failed to log payment data for %s, %v Shannon, tx: %s: %v", login, amount, txHash, err)
				u.halt = true
				u.lastFail = err
			break
		}

		minersPaid++
		totalAmount.Add(totalAmount, big.NewInt(amount))
		log.Printf("Paid %v Shannon to Standard Address: %v, TxHash: %v", amount, login, txHash)
	}
}

		    for login, amount := range tempAmountsIntegrated {
		      // Lock payments for current payout
		      err = u.backend.LockPayouts(login, amount)
		      if err != nil {
		        log.Printf("Failed to lock payment for %s: %v", login, err)
		        u.halt = true
		        u.lastFail = err
		        break
		      }
		      log.Printf("Locked payment for %s, %v Shannon", login, amount)

					// Debit miner's balance and update stats
		      err = u.backend.UpdateBalance(login, amount)
		      if err != nil {
        log.Printf("Failed to update balance for %s, %v Shannon: %v", login, amount, err)
		        u.halt = true
		        u.lastFail = err
		        break
		      }

					integdest := integdests[login]
		      txHash, err := u.rpc_wallet.SendTransaction([]rpc.TransferDestination{integdest}, baseFee, 0)
		      if err != nil {
						log.Printf("Failed to send payment to %v. Check outgoing tx for %s in block explorer and docs/PAYOUTS.md",
		          integdest, txHash)
		        u.halt = true
		        u.lastFail = err
		        return
		      }

		      // Log transaction hash
		      err = u.backend.WritePayment(login, txHash, amount)
		      if err != nil {
		        log.Printf("Failed to log payment data for %s, %v Shannon, tx: %s: %v", login, amount, txHash, err)
		       u.halt = true
		       u.lastFail = err
		       break
		     }

		      minersPaid++
		      totalAmount.Add(totalAmount, big.NewInt(amount))
		      log.Printf("Paid %v Shannon to Integrated Address: %v, TxHash: %v", amount, login, txHash)
		    }
		  }



	if mustPay > 0 {
		log.Printf("Paid total %v Shannon to %v of %v payees", totalAmount, minersPaid, mustPay)
	} else {
		log.Println("No payees that have reached payout threshold")
	}

	// Save redis state to disk
	if minersPaid > 0 && u.config.BgSave {
		u.bgSave()
	}
}

func (self PayoutsProcessor) isUnlockedAccount() bool {
	_, err := self.rpc_wallet.Sign(self.config.Address, "0x0")
	if err != nil {
		log.Println("Unable to process payouts:", err)
		return false
	}
	return true
}

func (self PayoutsProcessor) checkPeers() bool {
	n, err := self.rpc_daemon.GetPeerCount()
	if err != nil {
		log.Println("Unable to start payouts, failed to retrieve number of peers from node:", err)
		return false
	}
	if n < self.config.RequirePeers {
		log.Println("Unable to start payouts, number of peers on a node is less than required", self.config.RequirePeers)
		return false
	}
	return true
}

func (self PayoutsProcessor) reachedThreshold(amount *big.Int, threshold int64) bool {
	return big.NewInt(threshold).Cmp(amount) < 0
}

func formatPendingPayments(list []*storage.PendingPayment) string {
	var s string
	for _, v := range list {
		s += fmt.Sprintf("\tAddress: %s, Amount: %v Shannon, %v\n", v.Address, v.Amount, time.Unix(v.Timestamp, 0))
	}
	return s
}

func (self PayoutsProcessor) bgSave() {
	result, err := self.backend.BgSave()
	if err != nil {
		log.Println("Failed to perform BGSAVE on backend:", err)
		return
	}
	log.Println("Saving backend state to disk:", result)
}

func (self PayoutsProcessor) resolvePayouts() {
	payments := self.backend.GetPendingPayments()

	if len(payments) > 0 {
		log.Printf("Will credit back following balances:\n%s", formatPendingPayments(payments))

		for _, v := range payments {
			err := self.backend.RollbackBalance(v.Address, v.Amount)
			if err != nil {
				log.Printf("Failed to credit %v Shannon back to %s, error is: %v", v.Amount, v.Address, err)
				return
			}
			log.Printf("Credited %v Shannon back to %s", v.Amount, v.Address)
		}
		err := self.backend.UnlockPayouts()
		if err != nil {
			log.Println("Failed to unlock payouts:", err)
			return
		}
	} else {
		log.Println("No pending payments to resolve")
	}

	if self.config.BgSave {
		self.bgSave()
	}
	log.Println("Payouts unlocked")
}

func (self PayoutsProcessor) mustResolvePayout() bool {
	v, _ := strconv.ParseBool(os.Getenv("RESOLVE_PAYOUT"))
	return v
}
