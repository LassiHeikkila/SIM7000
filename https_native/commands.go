package https

import (
	"errors"
)

func checkNoErrorAndResponseOK(r []string, err error) error {
	if err != nil {
		return err
	}
	ok := false
	err2 := parseBasicOkOrError(r, &ok)
	if err2 != nil {
		return err2
	}
	if !ok {
		return errors.New("Response does contain \"OK\"")
	}
	return nil
}
