package tools

import (
	"fmt"
	"strings"
)

func isMissingCDNFileError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.TrimSpace(err.Error())
	return strings.HasPrefix(message, "404 ") || strings.HasPrefix(message, "410 ")
}

func doc2TextToolOutput(toolName string, fileUID string, err error) string {
	message := "text could not be retrieved from the CDN/doc2text service"
	if isMissingCDNFileError(err) {
		message = "the file is missing in the CDN, so no text could be retrieved"
	}
	return fmt.Sprintf("%s: file uid '%s': %s. Continue with other tools or approaches.", toolName, fileUID, message)
}

func lookupToolErrorOutput(toolName string, err error) string {
	return fmt.Sprintf("%s: lookup could not be completed: %v. Continue with other tools or approaches.", toolName, err)
}

func transcriptNotFoundToolOutput(contentUnitID string, language string) string {
	if language != "" {
		return fmt.Sprintf("transcript_lookup: no transcript was found for content_unit_id '%s' and language '%s'. Continue with other tools or approaches.", contentUnitID, language)
	}
	return fmt.Sprintf("transcript_lookup: no transcript was found for content_unit_id '%s'. Continue with other tools or approaches.", contentUnitID)
}
