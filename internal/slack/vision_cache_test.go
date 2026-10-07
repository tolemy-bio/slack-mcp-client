package slackbot

import (
	"context"
	"errors"
	"testing"
)

type countingDescriber struct {
	calls int
	err   error
}

func (d *countingDescriber) DescribeImage(_ context.Context, _ string, _ []byte) (string, error) {
	d.calls++
	if d.err != nil {
		return "", d.err
	}
	return "described", nil
}

func TestDescriptionCacheReusesSuccessfulDescriptions(t *testing.T) {
	inner := &countingDescriber{}
	d := WithDescriptionCache(inner)
	img := []byte("same-bytes")
	for i := 0; i < 3; i++ {
		got, err := d.DescribeImage(context.Background(), "image/png", img)
		if err != nil || got != "described" {
			t.Fatalf("call %d: got %q, %v", i, got, err)
		}
	}
	if inner.calls != 1 {
		t.Fatalf("expected 1 vision call, got %d", inner.calls)
	}
	if _, err := d.DescribeImage(context.Background(), "image/png", []byte("other")); err != nil || inner.calls != 2 {
		t.Fatalf("different image should call vision again: calls=%d err=%v", inner.calls, err)
	}
}

func TestDescriptionCacheDoesNotCacheFailures(t *testing.T) {
	inner := &countingDescriber{err: errors.New("boom")}
	d := WithDescriptionCache(inner)
	for i := 0; i < 2; i++ {
		if _, err := d.DescribeImage(context.Background(), "image/png", []byte("x")); err == nil {
			t.Fatal("expected error")
		}
	}
	if inner.calls != 2 {
		t.Fatalf("failures must not be cached, got %d calls", inner.calls)
	}
}
