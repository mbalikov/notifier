package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/PaesslerAG/jsonpath"
)

/*
 * Receives input string and replaces {{JSONPath}} with actual value
 */
func replaceJSONPathTags(msg_ctx *MessageContext, input string, tags_pp **[]string) string {
	if *tags_pp == nil {
		tagList := findTags(input)
		*tags_pp = &tagList
	}
	tags := *tags_pp
	if len(*tags) == 0 {
		return input
	}

	json_data := msg_ctx.JsonRpc.Params

	output := input
	for _, tag := range *tags {
		val, ok := msg_ctx.JSONPath_Cache[tag]
		if !ok {
			tag_val, err := jsonpath.Get(tag, json_data)
			if err != nil {
				log.Printf("JSONPath: fail to resolve tag \"%s\" in string \"%s\" : %s", tag, input, err)
				continue
			}

			val, ok = tag_val.(string)
			if !ok {
				val_bytes, err := json.Marshal(tag_val)
				if err != nil {
					log.Printf("JSONPath(%s): json encode failed value %v : %s", tag, val_bytes, err)
					continue
				}
				val = string(val_bytes)
			}
			msg_ctx.JSONPath_Cache[tag] = val
		}

		output = strings.Replace(output, "{{"+tag+"}}", val, -1)
	}

	return output
}

// findTags returns a slice of all unique tags found in the input string.
func findTags(input string) []string {
	tags := make(map[string]bool) // Use a map to store unique tags
	startIndex := 0

	for {
		openIndex := strings.Index(input[startIndex:], "{{")
		if openIndex == -1 {
			break
		}
		openIndex += startIndex // Adjust openIndex relative to the start of the string

		closeIndex := strings.Index(input[openIndex:], "}}")
		if closeIndex == -1 {
			break
		}
		closeIndex += openIndex + 2 // Adjust closeIndex and account for the length of "{{"

		tag := input[openIndex+2 : closeIndex-2]
		startIndex = closeIndex // Move startIndex to the end of the current tag

		if tag != "" {
			tags[tag] = true
		}
	}

	if len(tags) == 0 {
		return nil
	}

	// Convert the map keys to a slice
	i := 0
	tagList := make([]string, len(tags))
	for tag := range tags {
		tagList[i] = tag
		i += 1
	}

	return tagList
}
