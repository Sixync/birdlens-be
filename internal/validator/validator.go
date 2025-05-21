package validator

import (
	"fmt"
	"net/mail"
	"reflect"
	"strconv"
	"strings"
)

func Validate(val any) error {
	v := reflect.ValueOf(val)

	// rewrite this
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i).String()
		fieldName := v.Type().Field(i).Name
		tags := v.Type().Field(i).Tag.Get("validate")
		rules := strings.Split(tags, ",")

		if tags == "" {
			continue
		}

		for _, rule := range rules {
			if err := applyRule(rule, field, fieldName); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyRule(rule, field, fieldName string) error {
	switch {
	case strings.HasPrefix(rule, "max="):
		max, err := strconv.Atoi(strings.TrimPrefix(rule, "max="))
		if err != nil {
			return err
		}
		if len(field) > max {
			return fmt.Errorf("%s should be at most %d characters", fieldName, max)
		}

	case strings.HasPrefix(rule, "min="):
		min, err := strconv.Atoi(strings.TrimPrefix(rule, "min="))
		if err != nil {
			return err
		}
		if len(field) < min {
			return fmt.Errorf("%s should be at least %d characters", fieldName, min)
		}

	case rule == "required":
		if len(field) == 0 {
			return fmt.Errorf("%s should not be empty", fieldName)
		}

	case strings.HasPrefix(rule, "maxNum="):
		max, err := strconv.Atoi(strings.TrimPrefix(rule, "maxNum="))
		if err != nil {
			return err
		}

		num, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("%s should be a number", fieldName)
		}

		if num > max {
			return fmt.Errorf("%v muxt be smaller than %v", fieldName, max)
		}

	case strings.HasPrefix(rule, "minNum="):
		min, err := strconv.Atoi(strings.TrimPrefix(rule, "minNum="))
		if err != nil {
			return err
		}

		num, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("%s should be a number", fieldName)
		}

		if num > min {
			return fmt.Errorf("%v muxt be larger than %v", fieldName, min)
		}

	case strings.HasPrefix(rule, "email"):
		_, err := mail.ParseAddress(field)
		if err != nil {
			return fmt.Errorf("%s should be a valid email address", fieldName)
		}

	default:
		return fmt.Errorf("invalid validation rule: %s", rule)
	}

	return nil
}
