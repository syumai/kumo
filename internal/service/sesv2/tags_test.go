package sesv2

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const (
	testTagName         = "campaign"
	testTagValue        = "spring"
	testFeedbackIDValue = "members"
	testInvalidTagValue = "spring sale"
)

func TestValidateMessageTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []MessageTag
		wantErr bool
	}{
		{name: "nil tags", tags: nil},
		{name: "valid tags", tags: []MessageTag{{Name: "campaign_id", Value: "spring-2026"}, {Name: "A9", Value: "b"}}},
		{name: "256 characters", tags: []MessageTag{{Name: strings.Repeat("n", 256), Value: strings.Repeat("v", 256)}}},
		{name: "feedback id a", tags: []MessageTag{{Name: feedbackIDTagNameA, Value: testFeedbackIDValue}}},
		{name: "feedback id b", tags: []MessageTag{{Name: feedbackIDTagNameB, Value: "football"}}},
		{name: "empty name", tags: []MessageTag{{Name: "", Value: "v"}}, wantErr: true},
		{name: "empty value", tags: []MessageTag{{Name: "n", Value: ""}}, wantErr: true},
		{name: "name with space", tags: []MessageTag{{Name: "my tag", Value: "v"}}, wantErr: true},
		{name: "value with dot", tags: []MessageTag{{Name: "n", Value: "v1.0"}}, wantErr: true},
		{name: "non-ASCII value", tags: []MessageTag{{Name: "n", Value: "キャンペーン"}}, wantErr: true},
		{name: "name over 256 characters", tags: []MessageTag{{Name: strings.Repeat("n", 257), Value: "v"}}, wantErr: true},
		{name: "value over 256 characters", tags: []MessageTag{{Name: "n", Value: strings.Repeat("v", 257)}}, wantErr: true},
		{name: "other ses prefix", tags: []MessageTag{{Name: "ses:configuration-set", Value: "v"}}, wantErr: true},
		{name: "feedback id value with colon", tags: []MessageTag{{Name: feedbackIDTagNameA, Value: "a:b"}}, wantErr: true},
		{name: "second tag invalid", tags: []MessageTag{{Name: "ok", Value: "ok"}, {Name: "bad!", Value: "v"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMessageTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateMessageTags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSendEmail_EmailTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []MessageTag
		wantTags []MessageTag
		wantErr  bool
	}{
		{name: "without tags", tags: nil, wantTags: nil},
		{
			name:     "records tags",
			tags:     []MessageTag{{Name: testTagName, Value: testTagValue}, {Name: feedbackIDTagNameA, Value: testFeedbackIDValue}},
			wantTags: []MessageTag{{Name: testTagName, Value: testTagValue}, {Name: feedbackIDTagNameA, Value: testFeedbackIDValue}},
		},
		{name: "invalid tag", tags: []MessageTag{{Name: testTagName, Value: testInvalidTagValue}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()

			_, err := storage.SendEmail(t.Context(), &SendEmailRequest{
				FromEmailAddress: testSenderEmail,
				Destination:      &Destination{ToAddresses: []string{testRecipientEmail}},
				Content: &EmailContent{Simple: &SimpleEmail{
					Subject: &Content{Data: "S"},
					Body:    &Body{Text: &Content{Data: "B"}},
				}},
				EmailTags: tt.tags,
			})

			if tt.wantErr {
				var apiErr *IdentityError
				if !errors.As(err, &apiErr) || apiErr.Code != errBadRequest {
					t.Fatalf("expected BadRequestException, got %v", err)
				}

				if len(storage.SentEmails) != 0 {
					t.Fatal("rejected sends must not be recorded")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if got := storage.SentEmails[0].EmailTags; !reflect.DeepEqual(got, tt.wantTags) {
				t.Fatalf("EmailTags = %#v, want %#v", got, tt.wantTags)
			}
		})
	}
}

func TestSendEmail_ClonesEmailTags(t *testing.T) {
	storage := NewMemoryStorage()
	tags := []MessageTag{{Name: testTagName, Value: testTagValue}}

	_, err := storage.SendEmail(t.Context(), &SendEmailRequest{
		FromEmailAddress: testSenderEmail,
		Destination:      &Destination{ToAddresses: []string{testRecipientEmail}},
		Content: &EmailContent{Simple: &SimpleEmail{
			Subject: &Content{Data: "S"},
			Body:    &Body{Text: &Content{Data: "B"}},
		}},
		EmailTags: tags,
	})
	if err != nil {
		t.Fatal(err)
	}

	tags[0].Value = "mutated"

	if got := storage.SentEmails[0].EmailTags[0].Value; got != testTagValue {
		t.Fatalf("stored tags alias the request: %q", got)
	}
}

func TestSendBulkEmail_EmailTags(t *testing.T) {
	defaultTags := []MessageTag{{Name: testTagName, Value: "default"}, {Name: "tier", Value: "free"}}
	replacementTags := []MessageTag{{Name: testTagName, Value: "replacement"}}

	tests := []struct {
		name            string
		defaultTags     []MessageTag
		replacementTags []MessageTag
		wantTags        []MessageTag
	}{
		{name: "neither default nor replacement", wantTags: nil},
		{name: "default tags only", defaultTags: defaultTags, wantTags: defaultTags},
		{name: "replacement tags only", replacementTags: replacementTags, wantTags: replacementTags},
		{name: "replacement replaces default entirely", defaultTags: defaultTags, replacementTags: replacementTags, wantTags: replacementTags},
		{name: "empty replacement falls back to default", defaultTags: defaultTags, replacementTags: []MessageTag{}, wantTags: defaultTags},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()
			req := newBulkSendFixture(t, storage)
			req.BulkEmailEntries = req.BulkEmailEntries[:1]
			req.DefaultEmailTags = tt.defaultTags
			req.BulkEmailEntries[0].ReplacementTags = tt.replacementTags

			resp, err := storage.SendBulkEmail(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}

			if resp.BulkEmailEntryResults[0].Status != statusSuccess {
				t.Fatalf("unexpected result: %#v", resp.BulkEmailEntryResults[0])
			}

			if got := storage.SentEmails[0].EmailTags; !reflect.DeepEqual(got, tt.wantTags) {
				t.Fatalf("EmailTags = %#v, want %#v", got, tt.wantTags)
			}
		})
	}
}

func TestSendBulkEmail_InvalidDefaultEmailTagsRejectsRequest(t *testing.T) {
	storage := NewMemoryStorage()
	req := newBulkSendFixture(t, storage)
	req.DefaultEmailTags = []MessageTag{{Name: testTagName, Value: testInvalidTagValue}}

	_, err := storage.SendBulkEmail(t.Context(), req)

	var apiErr *IdentityError
	if !errors.As(err, &apiErr) || apiErr.Code != errBadRequest {
		t.Fatalf("expected BadRequestException, got %v", err)
	}

	if len(storage.SentEmails) != 0 {
		t.Fatal("rejected sends must not be recorded")
	}
}

func TestSendBulkEmail_InvalidReplacementTagsFailEntryIndividually(t *testing.T) {
	storage := NewMemoryStorage()
	req := newBulkSendFixture(t, storage)
	req.BulkEmailEntries[0].ReplacementTags = []MessageTag{{Name: testTagName, Value: testInvalidTagValue}}

	resp, err := storage.SendBulkEmail(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}

	if got := resp.BulkEmailEntryResults[0]; got.Status != statusFailed || got.Error == "" || got.MessageID != "" {
		t.Fatalf("entry 0: expected FAILED, got %#v", got)
	}

	if got := resp.BulkEmailEntryResults[1]; got.Status != statusSuccess {
		t.Fatalf("entry 1: expected SUCCESS, got %#v", got)
	}

	if len(storage.SentEmails) != 1 || storage.SentEmails[0].MessageID != resp.BulkEmailEntryResults[1].MessageID {
		t.Fatalf("only the valid entry must be recorded: %#v", storage.SentEmails)
	}
}
