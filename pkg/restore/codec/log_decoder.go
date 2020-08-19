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

package codec

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"

	"github.com/pingcap/errors"
	"github.com/pingcap/log"
	timodel "github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"go.uber.org/zap"
)

type ColumnFlagType uint64

const (
	// BatchVersion1 represents the version of batch format
	BatchVersion1 uint64 = 1
	// BinaryFlag means the column charset is binary
	BinaryFlag ColumnFlagType = 1 << ColumnFlagType(iota)
	// HandleKeyFlag means the column is selected as the handle key
	HandleKeyFlag
	// GeneratedColumnFlag means the column is a generated column
	GeneratedColumnFlag
	// PrimaryKeyFlag means the column is primary key
	PrimaryKeyFlag
	// UniqueKeyFlag means the column is unique key
	UniqueKeyFlag
	// MultipleKeyFlag means the column is multiple key
	MultipleKeyFlag
	// NullableFlag means the column is nullable
	NullableFlag
)

type column struct {
	Type byte `json:"t"`

	// WhereHandle is deprecation
	// WhereHandle is replaced by HandleKey in Flag
	WhereHandle *bool                `json:"h,omitempty"`
	Flag        ColumnFlagType `json:"f"`
	Value       interface{}          `json:"v"`
}


func formatColumnVal(c column) column {
	switch c.Type {
	case mysql.TypeTinyBlob, mysql.TypeMediumBlob,
		mysql.TypeLongBlob, mysql.TypeBlob:
		if s, ok := c.Value.(string); ok {
			var err error
			c.Value, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				log.Fatal("invalid column value, please report a bug", zap.Any("col", c), zap.Error(err))
			}
		}
	case mysql.TypeBit:
		if s, ok := c.Value.(json.Number); ok {
			intNum, err := s.Int64()
			if err != nil {
				log.Fatal("invalid column value, please report a bug", zap.Any("col", c), zap.Error(err))
			}
			c.Value = uint64(intNum)
		}
	}
	return c
}

type messageKey struct {
	Ts        uint64              `json:"ts"`
	Schema    string              `json:"scm,omitempty"`
	Table     string              `json:"tbl,omitempty"`
	Partition *int64              `json:"ptn,omitempty"`
}

func (m *messageKey) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m *messageKey) Decode(data []byte) error {
	return json.Unmarshal(data, m)
}


type messageDDL struct {
	Query string             `json:"q"`
	Type  timodel.ActionType `json:"t"`
}

func (m *messageDDL) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m *messageDDL) Decode(data []byte) error {
	return json.Unmarshal(data, m)
}

type messageRow struct {
	Update     map[string]column `json:"u,omitempty"`
	PreColumns map[string]column `json:"p,omitempty"`
	Delete     map[string]column `json:"d,omitempty"`
}

func (m *messageRow) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m *messageRow) Decode(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err := decoder.Decode(m)
	if err != nil {
		return errors.Trace(err)
	}
	for colName, column := range m.Update {
		m.Update[colName] = formatColumnVal(column)
	}
	for colName, column := range m.Delete {
		m.Delete[colName] = formatColumnVal(column)
	}
	for colName, column := range m.PreColumns {
		m.PreColumns[colName] = formatColumnVal(column)
	}
	return nil
}

// JSONEventBatchMixedDecoder decodes the byte of a batch into the original messages.
type JSONEventBatchMixedDecoder struct {
	mixedBytes []byte

	offset int
}

// NextRowChangedEvent implements the EventBatchDecoder interface
func (b *JSONEventBatchMixedDecoder) NextRowChangedEvent() (*kvPair, error) {
	if !b.hasNext() {
		log.Info("decode events finished")
		return nil, nil
	}
	keyLen := binary.BigEndian.Uint64(b.mixedBytes[:8])
	key := b.mixedBytes[8 : keyLen+8]
	b.mixedBytes = b.mixedBytes[keyLen+8:]

	valueLen := binary.BigEndian.Uint64(b.mixedBytes[:8])
	value := b.mixedBytes[8 : valueLen+8]
	b.mixedBytes = b.mixedBytes[valueLen+8:]

	keyMsg := new(messageKey)
	if err := keyMsg.Decode(key); err != nil {
		return nil, errors.Trace(err)
	}
	rowMsg := new(messageRow)
	if err := rowMsg.Decode(value); err != nil {
		return nil, errors.Trace(err)
	}

	// TODO spell to kv pair
	return nil, nil
}

// NextDDLEvent implements the EventBatchDecoder interface
func (b *JSONEventBatchMixedDecoder) NextDDLEvent() (string, error) {
	if !b.hasNext() {
		log.Info("decode events finished")
		return "", nil
	}
	keyLen := binary.BigEndian.Uint64(b.mixedBytes[:8])
	key := b.mixedBytes[8 : keyLen+8]
	b.mixedBytes = b.mixedBytes[keyLen+8:]

	b.mixedBytes = b.mixedBytes[keyLen+8:]
	valueLen := binary.BigEndian.Uint64(b.mixedBytes[:8])
	value := b.mixedBytes[8 : valueLen+8]
	b.mixedBytes = b.mixedBytes[valueLen+8:]

	ddlMsg := new(messageDDL)
	if err := ddlMsg.Decode(value); err != nil {
		return nil, errors.Trace(err)

	}
	// TODO spell to sql string with ts
	return "", nil
}

func (b *JSONEventBatchMixedDecoder) hasNext() bool {
	return len(b.mixedBytes) > 0
}

// NewJSONEventBatchDecoder creates a new JSONEventBatchDecoder.
func NewJSONEventBatchDecoder(data []byte) (*JSONEventBatchMixedDecoder, error) {
	version := binary.BigEndian.Uint64(data[:8])
	data = data[8:]
	if version != BatchVersion1 {
		return nil, errors.New("unexpected key format version")
	}
	return &JSONEventBatchMixedDecoder{
		mixedBytes: data,
	}, nil
}
