//go:build integration

package integration

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"
)

type sentEmailTag struct {
	Name  string
	Value string
}

func sentEmailTags(t *testing.T, messageID string) []sentEmailTag {
	t.Helper()

	resp, err := http.Get(testEndpoint() + "/kumo/ses/v2/sent-emails")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		SentEmails []struct {
			MessageID string `json:"MessageId"`
			EmailTags []sentEmailTag
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	for _, email := range result.SentEmails {
		if email.MessageID == messageID {
			return email.EmailTags
		}
	}

	t.Fatalf("sent message %q not found", messageID)

	return nil
}

func TestSESv2_SendEmail_EmailTags(t *testing.T) {
	client := newSESv2Client(t)

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String("tags@example.com"),
		Destination:      &types.Destination{ToAddresses: []string{"recipient@example.com"}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String("Tagged")},
			Body:    &types.Body{Text: &types.Content{Data: aws.String("Tagged body")}},
		}},
		EmailTags: []types.MessageTag{
			{Name: aws.String("campaign"), Value: aws.String("spring-2026")},
			{Name: aws.String("ses:feedback-id-a"), Value: aws.String("members")},
		},
	}

	output, err := client.SendEmail(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	want := []sentEmailTag{{Name: "campaign", Value: "spring-2026"}, {Name: "ses:feedback-id-a", Value: "members"}}
	if got := sentEmailTags(t, aws.ToString(output.MessageId)); !reflect.DeepEqual(got, want) {
		t.Fatalf("EmailTags = %#v, want %#v", got, want)
	}

	input.EmailTags = []types.MessageTag{{Name: aws.String("campaign"), Value: aws.String("spring sale")}}
	_, err = client.SendEmail(t.Context(), input)

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "BadRequestException" {
		t.Fatalf("expected BadRequestException, got %v", err)
	}
}

func TestSESv2_SendBulkEmail_EmailTags(t *testing.T) {
	client := newSESv2Client(t)
	name := "bulk-email-tags"
	createRenderingTemplate(t, client, name)

	destination := &types.Destination{ToAddresses: []string{"recipient@example.com"}}
	input := &sesv2.SendBulkEmailInput{
		FromEmailAddress: aws.String("bulk-tags@example.com"),
		DefaultContent: &types.BulkEmailContent{Template: &types.Template{
			TemplateName: aws.String(name),
			TemplateData: aws.String(`{"contact":{"name":"Default"}}`),
		}},
		DefaultEmailTags: []types.MessageTag{
			{Name: aws.String("campaign"), Value: aws.String("default")},
			{Name: aws.String("tier"), Value: aws.String("free")},
		},
		BulkEmailEntries: []types.BulkEmailEntry{
			{Destination: destination},
			{
				Destination:     destination,
				ReplacementTags: []types.MessageTag{{Name: aws.String("campaign"), Value: aws.String("replacement")}},
			},
			{
				Destination:     destination,
				ReplacementTags: []types.MessageTag{{Name: aws.String("campaign"), Value: aws.String("spring sale")}},
			},
		},
	}

	output, err := client.SendBulkEmail(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if len(output.BulkEmailEntryResults) != 3 {
		t.Fatalf("expected three entry results, got %d", len(output.BulkEmailEntryResults))
	}

	for i, want := range []types.BulkEmailStatus{types.BulkEmailStatusSuccess, types.BulkEmailStatusSuccess, types.BulkEmailStatusFailed} {
		if got := output.BulkEmailEntryResults[i].Status; got != want {
			t.Fatalf("entry %d status %q, want %q", i, got, want)
		}
	}

	wantDefault := []sentEmailTag{{Name: "campaign", Value: "default"}, {Name: "tier", Value: "free"}}
	if got := sentEmailTags(t, aws.ToString(output.BulkEmailEntryResults[0].MessageId)); !reflect.DeepEqual(got, wantDefault) {
		t.Fatalf("entry 0 EmailTags = %#v, want %#v", got, wantDefault)
	}

	wantReplacement := []sentEmailTag{{Name: "campaign", Value: "replacement"}}
	if got := sentEmailTags(t, aws.ToString(output.BulkEmailEntryResults[1].MessageId)); !reflect.DeepEqual(got, wantReplacement) {
		t.Fatalf("entry 1 EmailTags = %#v, want %#v", got, wantReplacement)
	}

	input.DefaultEmailTags = []types.MessageTag{{Name: aws.String("campaign"), Value: aws.String("spring sale")}}
	_, err = client.SendBulkEmail(t.Context(), input)

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "BadRequestException" {
		t.Fatalf("expected BadRequestException, got %v", err)
	}
}
