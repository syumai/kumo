package sesv2

import (
	"fmt"
	"regexp"
)

// messageTagPattern is the MessageTag Name and Value constraint: ASCII
// letters, numbers, underscores, or dashes, at most 256 characters. Both
// members are required, so an empty string is rejected as well.
// https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_MessageTag.html
var messageTagPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

const (
	feedbackIDTagNameA = "ses:feedback-id-a"
	feedbackIDTagNameB = "ses:feedback-id-b"
)

// feedbackIDTagNames are the ses:-prefixed tag names a caller may set even
// though the colon falls outside messageTagPattern. SES appends their values
// to the Feedback-ID header.
// https://docs.aws.amazon.com/ses/latest/dg/monitor-using-event-publishing.html#event-publishing-fine-grained-feedback
var feedbackIDTagNames = map[string]bool{
	feedbackIDTagNameA: true,
	feedbackIDTagNameB: true,
}

// validateMessageTags checks every tag against the MessageTag constraints and
// returns a message describing the first violation.
func validateMessageTags(tags []MessageTag) error {
	for _, tag := range tags {
		if !feedbackIDTagNames[tag.Name] && !messageTagPattern.MatchString(tag.Name) {
			return fmt.Errorf("invalid MessageTag Name %q: must be 1-256 ASCII letters, numbers, underscores, or dashes", tag.Name)
		}

		if !messageTagPattern.MatchString(tag.Value) {
			return fmt.Errorf("invalid MessageTag Value %q: must be 1-256 ASCII letters, numbers, underscores, or dashes", tag.Value)
		}
	}

	return nil
}

// bulkEmailTags returns the tags recorded for a bulk entry. AWS does not
// document how DefaultEmailTags and ReplacementTags combine; kumo treats
// non-empty ReplacementTags as authoritative and replaces the default tags
// entirely, mirroring how ReplacementTemplateData replaces the default data.
// https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_BulkEmailEntry.html
func bulkEmailTags(req *SendBulkEmailRequest, entry *BulkEmailEntry) []MessageTag {
	if len(entry.ReplacementTags) > 0 {
		return cloneMessageTags(entry.ReplacementTags)
	}

	return cloneMessageTags(req.DefaultEmailTags)
}

// cloneMessageTags returns a copy of tags to avoid aliasing the request.
func cloneMessageTags(tags []MessageTag) []MessageTag {
	if len(tags) == 0 {
		return nil
	}

	return append([]MessageTag{}, tags...)
}
