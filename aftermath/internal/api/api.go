package api

import (
	"aftermath/internal/database"
	"fmt"
)

type Cache struct {
	db *database.DB
}

func NewCache(db *database.DB) Cache {
	return Cache{db: db}
}

type CacheParams struct {
	Zettel string `json:"zettel"`
}

type CacheResult struct {
	Zettels []string `json:"zettels"`
	Error   string   `json:"error"`
}

func (c *Cache) GetAll(params *CacheParams, result *CacheResult) error {
	zettels, err := c.db.GetAllZettels()
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}

func (c *Cache) GetForwardLinks(params *CacheParams, result *CacheResult) error {
	zettels, err := c.db.GetForwardLinks(params.Zettel)
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}

func (c *Cache) GetBackLinks(params *CacheParams, result *CacheResult) error {
	zettels, err := c.db.GetBackLinks(params.Zettel)
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}
