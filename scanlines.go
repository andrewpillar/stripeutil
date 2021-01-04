package stripeutil

import (
	"bufio"
	"io"
)

func skipline(br *bufio.Reader) error {
	r, _, err := br.ReadRune()

	for {
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if r == '\n' {
			break
		}
		r, _, err = br.ReadRune()
	}
	return nil
}

func scanline(br *bufio.Reader, fn func(string)) error {
	lit := make([]rune, 0)

	r, _, err := br.ReadRune()

	for {
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if r == '\n' {
			break
		}
		lit = append(lit, r)
		r, _, err = br.ReadRune()
	}

	fn(string(lit))
	return nil
}

// scanlines will scan in the lines from the given io.Reader, and pass each
// line it successfully scans into the given callback. This will skip over
// whitespace, and ignore comments (lines prefixed with #).
func scanlines(rd io.Reader, fn func(string)) error {
	br := bufio.NewReader(rd)

	for {
redo:
		r, _, err := br.ReadRune()

		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			goto redo
		}

		if r == '#' {
			if err := skipline(br); err != nil {
				return err
			}
			goto redo
		}

		br.UnreadRune()

		if err := scanline(br, fn); err != nil {
			return err
		}
	}
	return nil
}
