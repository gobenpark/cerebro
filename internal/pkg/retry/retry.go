package retry

import "time"

func Retry(max int, f func() error) error {
	retries := 0
	for {
		if err := f(); err != nil {
			<-time.After((1 << retries) * time.Second)
			retries += 1

			if retries >= max {
				return err
			}
			continue
		}
		return nil
	}
}
