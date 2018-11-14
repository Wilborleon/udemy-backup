package backup

import (
	"context"
	"errors"
	"os"

	"github.com/ushu/udemy-backup/backup/config"
	"github.com/ushu/udemy-backup/cli"

	"github.com/ushu/udemy-backup/client"
)

func BackupCourse(ctx context.Context, course *client.Course) error {
	cfg, ok := config.FromContext(ctx)
	if !ok {
		return errors.New("missing config")
	}

	// then we list all the lectures for the course
	lectures, err := cfg.Client.LoadFullCurriculum(course.ID)
	if err != nil {
		cli.Log("☠️")
		return err
	}
	cli.Logf("⚙️  Found %d lectures for coure %s\n", len(lectures), course.Title)

	// we start by creating the necessary directories to hold all the lectures
	// the root dir
	if err := buildCourseDirectory(cfg, course); err != nil {
		return err
	}
	// and the dirs for all the chapters
	var currentChapter *client.Chapter
	for _, l := range lectures {
		if l.Chapter != nil && l.Chapter != currentChapter {
			if err := buildChapterDirectory(cfg, course, l.Chapter); err != nil {
				return err
			}
		}
	}

	// finally we enqueue the download work for all the lectures
	for _, lecture := range lectures {
		if err := BackupLecture(ctx, course, lecture); err != nil {
			return err
		}
	}
	return nil
}

func buildCourseDirectory(cfg *config.Config, course *client.Course) error {
	p := getCourseDirectory(cfg, course)
	return os.MkdirAll(p, 0755)
}

func buildChapterDirectory(cfg *config.Config, course *client.Course, chapter *client.Chapter) error {
	p := getChapterDirectory(cfg, course, chapter)
	return os.MkdirAll(p, 0755)
}