package testutil

import "strconv"

func DescribeUint(p *uint) string {
	if p == nil {
		return "nil"
	}
	return strconv.FormatUint(uint64(*p), 10)
}
