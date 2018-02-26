package models

import (
	"errors"
	"fmt"

	"github.com/go-redis/redis"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidLogin = errors.New("invalid login")
	UserExists      = errors.New("username exists")
)

//User key to the hashmap of stored attributes in redis
type User struct {
	id int64 //changes getting id from redis to directly from struct
	// key string
}

//NewUser constructor FUNCTION of User struct
func NewUser(username string, hash []byte) (*User, error) {
	exists, err := client.HExists("user:by-username", username).Result()
	//race condition exist if two clients registers at the same time: fixable by using locking to 1 channel for registeration
	//check if the user exists already before creating a new user; from user:by-username because it's shorts
	if exists {
		return nil, UserExists
	}
	id, err := client.Incr("user:next-id").Result() //assign id to assignment to redis
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("user:%d", id) //prefix id to create distinct namespace
	pipe := client.Pipeline()         //bundles the attributes in 1 set/get via network
	pipe.HSet(key, "id", id)          //a hashmap with its own key parameter; here redis key is assigned to hashmap key
	pipe.HSet(key, "username", username)
	pipe.HSet(key, "hash", hash)
	pipe.HSet("user:by-username", username, id) //redis object; contains a lookup with coorelated username and id;
	_, err = pipe.Exec()                        //execute pipeline: returns status (not important) and err
	if err != nil {
		return nil, err
	}

	// return &User{key}, nil //a new user
	return &User{id}, nil //a new user

}

//GetID gets user attribute from redis key
func (user *User) GetID() (int64, error) {
	return user.id, nil
	// return client.HGet(user.key, "id").Int64() //result is default: gives string
}

//GetUsername gets user attribute from redis key
func (user *User) GetUsername() (string, error) {
	key := fmt.Sprintf("user:%d", user.id)
	return client.HGet(key, "username").Result() //result is default: gives string
}

//GetHash gets user attribute from redis key
func (user *User) GetHash() ([]byte, error) {
	key := fmt.Sprintf("user:%d", user.id)
	return client.HGet(key, "hash").Bytes()
}

//Authenticate get registered hash from redis and compare it with entry
func (user *User) Authenticate(password string) error {
	hash, err := user.GetHash()
	if err != nil {
		return err //redis error from getting hash
	}
	err = bcrypt.CompareHashAndPassword(hash, []byte(password)) //does the compare return err
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return ErrInvalidLogin //if password incorrect
	}
	return err
}

//GetUserbyName returns a redis key based on username
func GetUserbyName(username string) (*User, error) {
	id, err := client.HGet("user:by-username", username).Int64()
	if err == redis.Nil {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}
	// key := fmt.Sprintf("user:%d", id)
	// return &User{key}, nil
	return GetUserbyID(id)
}

//GetUserbyID returns a redis user base on user_id
func GetUserbyID(id int64) (*User, error) {
	// key := fmt.Sprintf("user:%d", id)
	return &User{id}, nil
}

//LoginUser get registered hash from redis and compare it with entry
func LoginUser(username string, password string) (*User, error) {
	// hash, err := client.Get("user:" + username).Bytes() //get the store hash from redis
	// if err == redis.Nil {
	// 	return ErrUserNotFound
	// } else if err != nil {
	// 	return err
	// }

	// err = bcrypt.CompareHashAndPassword(hash, []byte(password)) //does the compare return err
	// if err != nil {
	// 	return ErrInvalidLogin //if password incorrect
	// }
	// return nil //in case no error, proceed to create session

	user, err := GetUserbyName(username)
	if err != nil {
		return nil, err
	}
	return user, user.Authenticate(password)

}

//RegisterUser hashes and stores the password with username as key;
func RegisterUser(username string, password string) error {
	cost := bcrypt.DefaultCost
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return err
	}
	err = client.Set("user:"+username, hash, 0).Err() //0 != expire

	_, err = NewUser(username, hash)

	return err

}
