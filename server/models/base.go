package models

import (
	"math"
	"time"

	"gorm.io/gorm"
)

const (
	MAX_PAGE_SIZE = 100
	MIN_PAGE_SIZE = 100
)

type BaseModel struct {
	ID        uint      `json:"id,omitempty" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type Paging struct {
	Total int64 `json:"total"`
	Page  int64 `json:"page"`
	Pages int64 `json:"pages"`
}

// ---------------------------------------------------------------------------------//
// Scopes
// --------------------------------------------------------------------------------//

func paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page == 0 {
			page = 1
		}

		switch {
		case pageSize > MAX_PAGE_SIZE:
			pageSize = MAX_PAGE_SIZE
		case pageSize <= 0:
			pageSize = MIN_PAGE_SIZE
		}

		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

func newPaging(page, pageSize, total int64) *Paging {
	paging := &Paging{Page: page, Total: total}
	if paging.Page == 0 {
		paging.Page = 1
	}

	paging.Pages = int64(math.Ceil(float64(paging.Total) / float64(pageSize)))
	if paging.Pages == 0 {
		paging.Pages = 1
	}

	return paging
}
