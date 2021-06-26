package proxy

import (
	"log"
	"math/big"
  "fmt"
	"strconv"
	"strings"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/common"
	"github.com/hostup/open-zano-pool/util"
)

var hasher = ethash.New()

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string) (bool, bool) {
	nonceHex := params[0]
	hashNoNonce := params[1]
	mixDigest := params[2]
	nonce, _ := strconv.ParseUint(strings.Replace(nonceHex, "0x", "", -1), 16, 64)
	shareDiff := s.config.Proxy.Difficulty

	  tempNonce, _ := new(big.Int).SetString(nonceHex[2:], 16)
	  tempNonceStr := []byte(fmt.Sprintf("%#018x", tempNonce)[2:])
	  flipped := make([]byte, 16)
	  for i := 0; i < 16; i += 2 {
	    flipped[16 - i - 1] = tempNonceStr[i + 1]
	    flipped[16 - i - 2] = tempNonceStr[i]
	  }

	h, ok := t.headers[hashNoNonce]
	if !ok {
		//TODO:Store stale share in Redis
		log.Printf("Stale share from %v@%v", login, ip)
		return false, false
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	block := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	share_params := []string{
    nonceHex,
    hashNoNonce,
    mixDigest,
    util.ToHexUint(share.NumberU64()),
    fmt.Sprintf("0x%x", share.Difficulty()),
  }

  good_share, err := s.rpc().VerifySolution(share_params)
  if err != nil {
    log.Printf("Error calling VerifySolution on share!")
  }

	if !*good_share {
		return false, false
	}

	//Write the Ip address into the settings:login:ipaddr and timeit added to settings:login:iptime hash
	s.backend.LogIP(login,ip)

	block_params := []string{
    nonceHex,
    hashNoNonce,
    mixDigest,
    util.ToHexUint(block.NumberU64()),
    fmt.Sprintf("0x%x", block.Difficulty()),
  }

  good_block, err := s.rpc().VerifySolution(block_params)
  if err != nil {
    log.Printf("Error calling VerifySolution on block!")
  }

	if *good_block {
    params := []string{t.Blob[2:4] + string(flipped) + t.Blob[20:]}
		ok, err := s.rpc().SubmitBlock(params)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, block_params[:3], shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {

                                return true, false
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, share_params[:3], shareDiff, h.height, s.hashrateExpiration)
		if exist {
			return true, false
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}
