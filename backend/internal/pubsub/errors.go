package pubsub

import "errors"

// ErrClosed is returned when operations are attempted on a closed PubSub
var ErrClosed = errors.New("pubsub: closed")
