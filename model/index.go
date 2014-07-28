package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boltdb/bolt"

	"../cfg"
)

const (
	BUCKET_INDEX = "index"
)

type Meta struct {
	Posts     int
	PerPage   int
	Pages     int
	Drafts    int
	UpdatedAt time.Time
}

func RebuildIndex() error {
	cfg.Log("Rebuilding index...")

	meta := Meta{PerPage: cfg.Int("ui.per_page"), UpdatedAt: time.Now(), Posts: 0, Pages: 1, Drafts: 0}
	pathMap := make(map[string]string)
	pageMap := make(map[string][]*Post)
	var draftList []*Post

	if err := db.View(func(tx *bolt.Tx) error {
		bucketPosts := tx.Bucket([]byte(BUCKET_POSTS))

		currentPage := 0
		pageKey := fmt.Sprintf("page-%d", currentPage)
		bucketPosts.ForEach(func(k, v []byte) error {
			post := new(Post)
			json.Unmarshal(v, &post)

			if post.Draft {
				meta.Drafts++
				draftList = append(draftList, post)
			} else {
				meta.Posts++
				pathMap[post.Path] = post.UUID

				pageMap[pageKey] = append(pageMap[pageKey], post)
				if len(pageMap[pageKey]) >= meta.PerPage {
					currentPage++
					meta.Pages++
					pageKey = fmt.Sprintf("page-%d", currentPage)
				}
			}

			return nil
		})

		return nil
	}); err != nil {
		return err
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(BUCKET_INDEX)); err != nil {
			return err
		}
		bucketIndex, err := tx.CreateBucketIfNotExists([]byte(BUCKET_INDEX))
		if err != nil {
			return err
		}
		if err := tx.DeleteBucket([]byte(BUCKET_MAP)); err != nil {
			return err
		}
		bucketMap, err := tx.CreateBucketIfNotExists([]byte(BUCKET_MAP))
		if err != nil {
			return err
		}

		jsonMeta, _ := json.Marshal(meta)
		cfg.Log(string(jsonMeta))
		if err := bucketIndex.Put([]byte("meta"), []byte(jsonMeta)); err != nil {
			return err
		}

		for pageKey, postsPage := range pageMap {
			jsonPage, _ := json.Marshal(postsPage)
			if err := bucketIndex.Put([]byte(pageKey), []byte(jsonPage)); err != nil {
				return err
			}
		}

		jsonDrafts, _ := json.Marshal(draftList)
		if err := bucketIndex.Put([]byte("drafts"), []byte(jsonDrafts)); err != nil {
			return err
		}

		for path, uuid := range pathMap {
			if err := bucketMap.Put([]byte(path), []byte(uuid)); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}