package main

import "errors"

var (
	errRoomNotFound  = errors.New("Room not found — check your code and try again")
	errRoomFull      = errors.New("This room is full")
	errGameInProgress = errors.New("A game is already in progress in this room")
)
