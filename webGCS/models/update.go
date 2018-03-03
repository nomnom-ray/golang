package models

import (
	"fmt"
	"strconv"
)

type Update struct {
	id int64
}

//NewUpdate constructor FUNCTION of Update struct
func NewUpdate(userID int64, content string) (*Update, error) {
	id, err := client.Incr("update:next-id").Result() //assign id to assignment to redis
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("update:%d", id) //prefix id to create distinct namespace
	pipe := client.Pipeline()           //bundles the attributes in 1 set/get via network
	pipe.HSet(key, "id", id)            //a hashmap with its own key parameter; here redis key is assigned to hashmap key
	pipe.HSet(key, "user_id", userID)
	pipe.HSet(key, "content", content)
	pipe.LPush("updates", id) //push the UserID instead of the actual content onto a list; use ID to get content

	pipe.LPush(fmt.Sprintf("user:%d:updates", userID), id) //a user specific update list

	_, err = pipe.Exec() //execute pipeline: returns status (not important) and err
	if err != nil {
		return nil, err
	}

	return &Update{id}, nil //a new update
}

//GetContent gets content
func (update *Update) GetContent() (string, error) {
	key := fmt.Sprintf("update:%d", update.id)
	return client.HGet(key, "content").Result()
}

// GetUser uses a User method to return a User with ID associated with an update
func (update *Update) GetUser() (*User, error) {
	key := fmt.Sprintf("update:%d", update.id)
	userID, err := client.HGet(key, "user_id").Int64()
	if err != nil {
		return nil, err
	}
	return GetUserbyID(userID)
}

// queryUpdates is a helper function to get updates
func queryUpdates(key string) ([]*Update, error) {
	updateIDs, err := client.LRange(key, 0, 10).Result()
	if err != nil {
		return nil, err
	}

	updates := make([]*Update, len(updateIDs)) //allocate memmory for update struct by each updateID
	for i, idString := range updateIDs {
		id, err := strconv.ParseInt(idString, 10, 64)
		if err != nil {
			return nil, err
		}
		updates[i] = &Update{id} //populate update memory space with update by key
	}
	return updates, nil
}

//GetGlobalUpdates returns updates as strings; 10==10 updates in a global scope
func GetGlobalUpdates() ([]*Update, error) {
	// 	updateIDs, err := client.LRange("updates", 0, 10).Result()
	//  //this name UpdateIDs gets ID instead of content because that's how it's stored
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	updates := make([]*Update, len(updateIDs)) //allocate memmory for update struct by each updateID
	// 	for i, id := range updateIDs {
	// 		key := "update:" + id
	// 		updates[i] = &Update{key} //populate update memory space with update by key
	// 	}
	// 	return updates, nil

	return queryUpdates("updates")

}

//GetUserUpdates returns updates as strings; per particular user
func GetUserUpdates(userID int64) ([]*Update, error) {
	key := fmt.Sprintf("user:%d:updates", userID)

	return queryUpdates(key)

	// updateIDs, err := client.LRange(key, 0, 10).Result()
	// if err != nil {
	// 	return nil, err
	// }

	// updates := make([]*Update, len(updateIDs)) //allocate memmory for update struct by each updateID
	// for i, id := range updateIDs {
	// 	key := "update:" + id
	// 	updates[i] = &Update{key} //populate update memory space with update by key
	// }
	// return updates, nil
}

//PostUpdates pushes to comments fails; must have the Err() method if assign err
func PostUpdates(userID int64, content string) error {
	// return client.LPush("updates", content).Err()

	_, err := NewUpdate(userID, content)
	return err
}
