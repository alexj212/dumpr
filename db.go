// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"sort"
	"time"
)

// SessionBucket bucket name for boltdb storage
const SessionBucket = "Sessions"

var (
	db *bolt.DB
)

// InitializeDB initialize boltdb, returns db reference and error
func InitializeDB() (*bolt.DB, error) {
	// Open the dumpr.db data file in your current directory.
	// It will be created if it doesn't exist.
	var err error

	filename := fmt.Sprintf("%s/dumpr.db", *saveDir)

	db, err := bolt.Open(filename, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		fmt.Printf("Unable to open boltdb error: %v\n", err)
		return nil, err
	}

	fmt.Printf("Opened dumpr.db data file\n")

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(SessionBucket))
		if err != nil {
			// ignore bucket already created error
		}
		return nil
	})

	return db, err
}

// StoreSession store a session within the boltdb bucket
func StoreSession(s *Session) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SessionBucket))
		err := b.Put([]byte(s.Key), s.Bytes())
		return err
	})
	return err
}

// LoadSession load a session within the boltdb bucket
func LoadSession(key string) (s *Session, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SessionBucket))
		raw := b.Get([]byte(key))

		if raw != nil {
			s = &Session{}
			err := json.Unmarshal(raw, s)
			if err != nil {
				return err
			}

			var valid bool
			valid, err = s.IsValid()
			if !valid || err != nil {
				return fmt.Errorf("resources missing from session")
			}

			if s.Protocol == HTTP {
				err = s.LoadHTTPRequestJSON()
				if err != nil {
					fmt.Printf("LoadSession: %s - error loading LoadHTTPRequestJSON %v\n", key, err)
				}
			}
		}
		return nil
	})

	return s, err
}

// DeleteSession delete a session within the boltdb bucket
func DeleteSession(key string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SessionBucket))
		return b.Delete([]byte(key))
	})

	return err
}

// LoadSessions load a list of sessions from the boltdb bucket
func LoadSessions() ([]*Session, error) {
	listSessions := make([]*Session, 0)
	invalidSessions := make([]*Session, 0)

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(SessionBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			s := &Session{}
			err := json.Unmarshal(v, s)
			if err == nil {
				valid, err := s.IsValid()
				if err != nil {
					invalidSessions = append(invalidSessions, s)
					fmt.Printf("Session File missing, removing bad session: %v\n", err)
					continue
				}

				if valid {
					s.Active = false
					listSessions = append(listSessions, s)
					Sessions[s.Key] = s
					fmt.Printf("Add valid session to InActiveSessions list: %v\n", s.Key)

					if s.Protocol == HTTP {
						err = s.LoadHTTPRequestJSON()
						if err != nil {
							fmt.Printf("LoadSession: %s - error loading LoadHTTPRequestJSON %v\n", s.Key, err)
						}
					}

				} else {
					invalidSessions = append(invalidSessions, s)
					fmt.Printf("session: %s, valid is false, removing bad session\n", s.Key)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	for _, s := range invalidSessions {
		err := DeleteSession(s.Key)
		if err != nil {
			fmt.Printf("Error deleting session: %s error: %v\n", s.Key, err)
		}
	}

	sort.Slice(listSessions, func(i, j int) bool {
		return listSessions[i].StartTime.Unix() < listSessions[j].StartTime.Unix()
	})

	fmt.Printf("LoadSessions len valid: %v invalid: %v\n", len(listSessions), len(invalidSessions))
	return listSessions, nil
}
