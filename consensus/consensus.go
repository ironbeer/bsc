// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package consensus implements different Ethereum consensus engines.
package consensus

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	SystemAddress = common.HexToAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")
)

// ChainHeaderReader defines a small collection of methods needed to access the local
// blockchain during header verification.
type ChainHeaderReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *types.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *types.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *types.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) *types.Header

	// GetHighestVerifiedHeader retrieves the highest header verified.
	GetHighestVerifiedHeader() *types.Header
}

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {
	ChainHeaderReader

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) *types.Block
}

// Engineは、アルゴリズムに依存しないコンセンサスエンジンです。
type Engine interface {
	// Authorは、与えられたブロックを鋳造したアカウントのEthereumアドレスを取得します。
	// ブロックを鋳造したアカウントのEthereumアドレスを取得しますが、コンセンサスエンジンが署名に基づいている場合、ヘッダーのコインベースとは異なる可能性があります。
	// これは、コンセンサスエンジンがシグネチャに基づいている場合、ヘッダのコインベースとは異なる可能性があります。
	Author(header *types.Header) (common.Address, error)

	// VerifyHeaderは、ヘッダがあるエンジンのコンセンサスルールに適合しているかどうかをチェックします。
	// 与えられたエンジンに適合しているかどうかをチェックします。シールの検証は、ここで任意に行うこともできますが、明示的に
	// VerifySeal メソッドで明示的に行うこともできます。
	VerifyHeader(chain ChainHeaderReader, header *types.Header, seal bool) error

	// VerifyHeaders は VerifyHeader と似ていますが、ヘッダのバッチを検証します。
	// 同時進行で検証します。このメソッドは，操作を中断するための quit チャネルと， // 検証結果を取得するための results チャネルを返します。
	// 非同期の検証結果を取得するための結果チャンネルを返します（順序は入力スライスの
	// 非同期検証を取得する結果チャネルを返します（順序は入力スライスの順序です）。)
	VerifyHeaders(chain ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error)

	// VerifyUncles は、与えられたブロックのアンクルが、与えられたエンジンのコンセンサス
	// 与えられたエンジンのコンセンサスルールに準拠しているかを検証します。
	VerifyUncles(chain ChainReader, block *types.Block) error

	// Prepare は、ブロックヘッダーのコンセンサスフィールドを、特定のエンジンのルールに従って初期化します。
	// 特定のエンジンのルールに従ってブロックヘッダのコンセンサスフィールドを初期化します。変更はインラインで実行されます。
	Prepare(chain ChainHeaderReader, header *types.Header) error

	// Finalizeは、トランザクション後の状態の変更（ブロックの報酬など）を実行します。
	// しかし、ブロックの組み立ては行いません。
	//
	// 注意：ブロックヘッダと状態データベースは、ファイナライズ時に発生したコンセンサスルールを反映するために更新される可能性があります。
	// ブロックヘッダと状態データベースは、ファイナライズ時に発生するコンセンサスルール（例：ブロックリワード）を反映するために更新されるかもしれません。
	Finalize(chain ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction,
		uncles []*types.Header, receipts *[]*types.Receipt, systemTxs *[]*types.Transaction, usedGas *uint64) error

	// FinalizeAndAssemble は、トランザクション後の状態の変更 (例：ブロック
	// 報酬など）を実行し、最終ブロックを組み立てます。
	//
	// 注意：ブロックヘッダと状態データベースは、最終的に発生したコンセンサスルールを反映するために更新されるかもしれません。
	// ブロックヘッダと状態データベースが更新され、最終処理で発生するコンセンサスルールが反映されるかもしれません。
	FinalizeAndAssemble(chain ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
		uncles []*types.Header, receipts []*types.Receipt) (*types.Block, []*types.Receipt, error)

	// Seal は、与えられた入力ブロックに対して新しいシーリング要求を生成し、その結果を与えられたチャネルにプッシュします。
	// その結果を与えられたチャネルにプッシュします。
	//
	// なお、このメソッドはすぐに戻り、結果は非同期に送信されます。さらに
	// コンセンサスアルゴリズムによっては、 // 複数の結果が返されることもあります。
	Seal(chain ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error

	// SealHashは、シールされる前のブロックのハッシュを返します。
	SealHash(header *types.Header) common.Hash

	// CalcDifficultyは、難易度調整アルゴリズムです。これは、新しいブロックが持つべき難易度
	// 新しいブロックが持つべき難易度を返します。
	CalcDifficulty(chain ChainHeaderReader, time uint64, parent *types.Header) *big.Int

	// APIs このコンセンサス・エンジンが提供する RPC API を返します。
	APIs(chain ChainHeaderReader) []rpc.API

	// Delay マイナーがtxsをコミットできる最大時間を返します。
	Delay(chain ChainReader, header *types.Header) *time.Duration

	// Close は、コンセンサスエンジンが維持しているバックグラウンドスレッドを終了させます。
	Close() error
}

// PoW is a consensus engine based on proof-of-work.
type PoW interface {
	Engine

	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
}

type PoSA interface {
	Engine

	IsSystemTransaction(tx *types.Transaction, header *types.Header) (bool, error)
	IsSystemContract(to *common.Address) bool
	EnoughDistance(chain ChainReader, header *types.Header) bool
	IsLocalBlock(header *types.Header) bool
	AllowLightProcess(chain ChainReader, currentHeader *types.Header) bool
}
