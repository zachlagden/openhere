package sites

import (
	"crypto/rand"
	"fmt"
)

const (
	suffixAlphabet = "abcdefghijklmnopqrstuvwxyz234567"
	suffixLength   = 4
	slugAttempts   = 8
)

var adjectives = []string{
	"amber", "bold", "brisk", "bright", "calm", "clever", "cosy", "crisp", "dapper", "eager", "early", "fancy",
	"gentle", "glad", "golden", "happy", "hazy", "jolly", "keen", "kind", "light", "lively", "lucky", "mellow",
	"merry", "mighty", "misty", "modest", "noble", "nimble", "plain", "polite", "proud", "quick", "quiet",
	"rapid", "rustic", "shiny", "silent", "silver", "smooth", "snug", "solid", "spry", "steady", "sunny",
	"swift", "tidy", "warm", "wild", "wise", "witty", "young", "zesty", "ancient", "azure", "breezy", "cheery",
	"copper", "dusty", "fresh", "gleaming", "humble", "ivory",
}

var nouns = []string{
	"acorn", "anchor", "aspen", "badger", "beacon", "birch", "bridge", "brook", "canyon", "cedar", "cliff",
	"clover", "comet", "coral", "creek", "dune", "eagle", "ember", "falcon", "fern", "field", "finch", "fjord",
	"forest", "garden", "glacier", "grove", "harbor", "hazel", "heron", "hill", "island", "juniper", "lagoon",
	"lantern", "maple", "meadow", "mesa", "moss", "oak", "orchard", "otter", "pebble", "pine", "pond", "prairie",
	"quartz", "raven", "reef", "ridge", "river", "robin", "sage", "shore", "sparrow", "spruce", "summit",
	"thicket", "trail", "tulip", "valley", "willow", "wren",
}

func RandomSlug() (string, error) {
	pick := make([]byte, 2+suffixLength)
	if _, err := rand.Read(pick); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	suffix := make([]byte, suffixLength)
	for i := range suffix {
		suffix[i] = suffixAlphabet[int(pick[2+i])%len(suffixAlphabet)]
	}
	adjective := adjectives[int(pick[0])%len(adjectives)]
	noun := nouns[int(pick[1])%len(nouns)]
	return adjective + "-" + noun + "-" + string(suffix), nil
}
