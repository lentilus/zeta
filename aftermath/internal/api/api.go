package api

import (
	"aftermath/internal/database"
	"aftermath/internal/scheduler"
	"fmt"
)

type Index struct {
	roDB *database.DB
	rwDB *database.DB
	s    *scheduler.Scheduler
}

func NewIndex(roDB *database.DB, rwDB *database.DB, s *scheduler.Scheduler) Index {
	return Index{roDB: roDB, rwDB: rwDB, s: s}
}

type CacheParams struct {
	Zettel string `json:"zettel"`
}

type CacheResult struct {
	Zettels []string `json:"zettels"`
	Error   string   `json:"error"`
}

func (c *Index) GetAll(params *CacheParams, result *CacheResult) error {
	zettels, err := c.roDB.GetAllZettels()
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}

func (c *Index) GetForwardLinks(params *CacheParams, result *CacheResult) error {
	zettels, err := c.roDB.GetForwardLinks(params.Zettel)
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}

func (c *Index) GetBackLinks(params *CacheParams, result *CacheResult) error {
	zettels, err := c.roDB.GetBackLinks(params.Zettel)
	result.Zettels = zettels
	result.Error = fmt.Sprint(err)
	return nil
}
