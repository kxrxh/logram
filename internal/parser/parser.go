package parser

func ParseLogStream(ch <-chan []byte) []string {
	var lines []string
	for line := range ch {
		lines = append(lines, string(line))
	}
	return lines
}
