// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"bytes"
	"testing"

	. "github.com/pingcap/check"
	"github.com/pingcap/log"
	"github.com/pingcap/tidb/types"
	"go.uber.org/zap"
)

type rowSuite struct{}

var _ = Suite(&rowSuite{})

func TestRow(t *testing.T) {
	TestingT(t)
}

func (s *rowSuite) TestMarshal(c *C) {
	dats := make([]types.Datum, 4)
	dats[0].SetInt64(1)
	dats[1].SetNull()
	dats[2] = types.MaxValueDatum()
	dats[3] = types.MinNotNullDatum()

	// save coverage for MarshalLogArray
	log.Info("row marshal", zap.Array("row", rowArrayMarshaler(dats)))
}

func (s *kvSuite) TestSimplePairIter(c *C) {
	pairs := []Pair{
		{Key: []byte("1"), Val: []byte("a")},
		{Key: []byte("2"), Val: []byte("b")},
		{Key: []byte("3"), Val: []byte("c")},
		{Key: []byte("5"), Val: []byte("d")},
	}
	iter := NewSimpleKeyIter(pairs)
	c.Assert(bytes.Equal(iter.First(), []byte("1")), IsTrue)
	c.Assert(bytes.Equal(iter.Last(), []byte("5")), IsTrue)

	c.Assert(iter.Seek([]byte("1")), IsTrue)
	c.Assert(bytes.Equal(iter.Key(), []byte("1")), IsTrue)
	c.Assert(bytes.Equal(iter.Value(), []byte("a")), IsTrue)
	c.Assert(iter.Valid(), IsTrue)

	c.Assert(iter.Seek([]byte("2")), IsTrue)
	c.Assert(bytes.Equal(iter.Key(), []byte("2")), IsTrue)
	c.Assert(bytes.Equal(iter.Value(), []byte("b")), IsTrue)
	c.Assert(iter.Valid(), IsTrue)

	c.Assert(iter.Seek([]byte("3")), IsTrue)
	c.Assert(bytes.Equal(iter.Key(), []byte("3")), IsTrue)
	c.Assert(bytes.Equal(iter.Value(), []byte("c")), IsTrue)
	c.Assert(iter.Valid(), IsTrue)

	// 4 not exists, so seek position will move to 5.
	c.Assert(iter.Seek([]byte("4")), IsTrue)
	c.Assert(bytes.Equal(iter.Key(), []byte("5")), IsTrue)
	c.Assert(bytes.Equal(iter.Value(), []byte("d")), IsTrue)
	c.Assert(iter.Valid(), IsTrue)

	// 6 not exists, so seek position will move to 5.
	c.Assert(iter.Seek([]byte("6")), IsFalse)
	c.Assert(bytes.Equal(iter.Key(), []byte("5")), IsTrue)
	c.Assert(bytes.Equal(iter.Value(), []byte("d")), IsTrue)
	c.Assert(iter.Valid(), IsTrue)
}
