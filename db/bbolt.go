package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xmdhs/clash2sfa/model"
	"go.etcd.io/bbolt"
)

type BBolt struct {
	db *bbolt.DB
}

func NewBBolt(path string) (*BBolt, error) {
	db, err := bbolt.Open(path, 0666, bbolt.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("NewBBolt: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("arg"))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("NewBBolt: %w", err)
	}
	return &BBolt{db: db}, nil
}

var ErrNotFind = errors.New("没找到")

func (b *BBolt) GetArg(cxt context.Context, blake3 string) (model.ConvertArg, error) {
	m := model.ConvertArg{}
	err := b.db.View(func(tx *bbolt.Tx) error {
		buc := tx.Bucket([]byte("arg"))
		b := buc.Get([]byte(blake3))
		if b == nil {
			return ErrNotFind
		}
		err := json.Unmarshal(b, &m)
		if err != nil {
			panic(err)
		}
		return nil
	})
	if err != nil {
		return m, fmt.Errorf("GetArg: %w", err)
	}
	return m, nil
}

func (b *BBolt) PutArg(cxt context.Context, blake3 string, arg model.ConvertArg) error {
	rb, err := json.Marshal(arg)
	if err != nil {
		return fmt.Errorf("PutArg: %w", err)
	}
	err = b.db.Update(func(tx *bbolt.Tx) error {
		buc := tx.Bucket([]byte("arg"))
		return buc.Put([]byte(blake3), rb)
	})
	if err != nil {
		return fmt.Errorf("PutArg: %w", err)
	}
	return nil
}
