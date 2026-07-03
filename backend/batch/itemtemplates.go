package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/freeeve/libcatalog/backend/store"
)

// ItemTemplate is a saved item field set (tasks/069): applied it pre-fills
// the item form; its barcode prefix seeds bulk add's auto-incrementing
// pattern. Personal or library-shared on the macros sharing model.
type ItemTemplate struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	CallNumber string `json:"callNumber,omitempty"`
	Location   string `json:"location,omitempty"`
	Note       string `json:"note,omitempty"`
	// BarcodePrefix seeds bulk add ("B-" -> B-0001, B-0002, ...).
	BarcodePrefix string `json:"barcodePrefix,omitempty"`
	// BarcodeWidth is the zero-padded counter width (default 4).
	BarcodeWidth int       `json:"barcodeWidth,omitempty"`
	Shared       bool      `json:"shared"`
	Owner        string    `json:"owner"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func itemTemplateKey(scope, id string) store.Key {
	return store.Key{PK: "ITMPL#" + scope, SK: "T#" + id}
}

func itemTemplateScope(t ItemTemplate) string {
	if t.Shared {
		return sharedPartition
	}
	return t.Owner
}

// CreateItemTemplate validates and stores a template for owner (in the
// shared partition when t.Shared). The id is minted server-side.
func (s *Service) CreateItemTemplate(ctx context.Context, t ItemTemplate, owner string) (ItemTemplate, error) {
	if err := validateItemTemplate(t); err != nil {
		return ItemTemplate{}, err
	}
	t.ID = mintID()
	t.Owner = owner
	now := time.Now().UTC()
	t.CreatedAt, t.UpdatedAt = now, now
	if err := s.putItemTemplate(ctx, t, store.CondIfAbsent); err != nil {
		return ItemTemplate{}, err
	}
	return t, nil
}

// UpdateItemTemplate replaces a template's definition. Only the owner may
// update; flipping Shared moves the record between partitions.
func (s *Service) UpdateItemTemplate(ctx context.Context, id string, t ItemTemplate, owner string) (ItemTemplate, error) {
	if err := validateItemTemplate(t); err != nil {
		return ItemTemplate{}, err
	}
	current, err := s.GetItemTemplate(ctx, owner, id)
	if err != nil {
		return ItemTemplate{}, err
	}
	if current.Owner != owner {
		return ItemTemplate{}, ErrForbidden
	}
	t.ID = current.ID
	t.Owner = current.Owner
	t.CreatedAt = current.CreatedAt
	t.UpdatedAt = time.Now().UTC()
	if current.Shared != t.Shared {
		if err := s.DB.Delete(ctx, store.Record{Key: itemTemplateKey(itemTemplateScope(current), current.ID)}, store.CondNone); err != nil && !errors.Is(err, store.ErrNotFound) {
			return ItemTemplate{}, err
		}
	}
	if err := s.putItemTemplate(ctx, t, store.CondNone); err != nil {
		return ItemTemplate{}, err
	}
	return t, nil
}

// DeleteItemTemplate removes an owned template (shared or personal).
func (s *Service) DeleteItemTemplate(ctx context.Context, owner, id string) error {
	t, err := s.GetItemTemplate(ctx, owner, id)
	if err != nil {
		return err
	}
	if t.Owner != owner {
		return ErrForbidden
	}
	err = s.DB.Delete(ctx, store.Record{Key: itemTemplateKey(itemTemplateScope(t), t.ID)}, store.CondNone)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// GetItemTemplate resolves a template the caller can use: their own, or a
// shared one.
func (s *Service) GetItemTemplate(ctx context.Context, owner, id string) (ItemTemplate, error) {
	for _, scope := range []string{owner, sharedPartition} {
		rec, err := s.DB.Get(ctx, itemTemplateKey(scope, id))
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return ItemTemplate{}, err
		}
		var t ItemTemplate
		if err := json.Unmarshal(rec.Data, &t); err != nil {
			return ItemTemplate{}, err
		}
		return t, nil
	}
	return ItemTemplate{}, ErrNotFound
}

// ListItemTemplates returns the caller's templates plus every shared one,
// sorted by label.
func (s *Service) ListItemTemplates(ctx context.Context, owner string) ([]ItemTemplate, error) {
	out := []ItemTemplate{}
	for _, scope := range []string{owner, sharedPartition} {
		for rec, err := range s.DB.Query(ctx, "ITMPL#"+scope, "T#", store.QueryOpt{}) {
			if err != nil {
				return nil, err
			}
			var t ItemTemplate
			if json.Unmarshal(rec.Data, &t) == nil {
				out = append(out, t)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Service) putItemTemplate(ctx context.Context, t ItemTemplate, cond store.Cond) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = s.DB.Put(ctx, store.Record{Key: itemTemplateKey(itemTemplateScope(t), t.ID), Data: data}, cond)
	return err
}

func validateItemTemplate(t ItemTemplate) error {
	if t.Label == "" {
		return fmt.Errorf("%w: an item template needs a label", ErrValidation)
	}
	if t.BarcodeWidth < 0 || t.BarcodeWidth > 12 {
		return fmt.Errorf("%w: barcode width must be 0-12", ErrValidation)
	}
	return nil
}
