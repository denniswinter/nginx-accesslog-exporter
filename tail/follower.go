package tail

import "github.com/hpcloud/tail"

// Follower describes an object that emits a stream of lines
type Follower interface {
	Lines() chan *tail.Line
	OnError(func(error))
}

type follower struct {
	filename string
	t        *tail.Tail
}

// NewFollower creates a new Follower instance for a given file
func NewFollower(filename string) (Follower, error) {
	f := &follower{
		filename: filename,
	}

	if err := f.start(); err != nil {
		return nil, err
	}

	return f, nil
}

func (f *follower) start() error {
	t, err := tail.TailFile(f.filename, tail.Config{
		Follow: true,
	})

	if err != nil {
		return err
	}

	f.t = t
	return nil
}

func (f *follower) OnError(cb func(error)) {
	go func() {
		err := f.t.Wait()
		if err != nil {
			cb(err)
		}
	}()
}

func (f *follower) Lines() chan *tail.Line {
	return f.t.Lines
}
