package sgroupbot

import "io"

func ReadAll(b []byte, r io.Reader) ([]byte, error) {
	if cap(b) == 0 {
		b = make([]byte, 0, 512)
	}
	for {
		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}
	}
}
