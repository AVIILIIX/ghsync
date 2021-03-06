package deep

import (
	"context"
	"database/sql"

	"github.com/src-d/ghsync/models"

	"github.com/google/go-github/github"
	"gopkg.in/src-d/go-kallax.v1"
	"gopkg.in/src-d/go-log.v1"
	"gopkg.in/src-d/go-queue.v1"
)

type RepositorySyncer struct {
	s *models.RepositoryStore
	c *github.Client
}

func NewRepositorySyncer(db *sql.DB, c *github.Client) *RepositorySyncer {
	return &RepositorySyncer{
		s: models.NewRepositoryStore(db),
		c: c,
	}
}

func (s *RepositorySyncer) QueueOrganization(q queue.Queue, owner string) error {
	opts := &github.RepositoryListByOrgOptions{}
	opts.ListOptions.PerPage = listOptionsPerPage

	logger := log.New(log.Fields{"type": RepositorySyncTask, "owner": owner})
	logger.Infof("starting to publish queue jobs")

	for {
		repositories, r, err := s.c.Repositories.ListByOrg(context.TODO(), owner, opts)
		if err != nil {
			return err
		}

		for _, r := range repositories {
			j, err := NewRepositorySyncJob(owner, r.GetName())
			if err != nil {
				return err
			}

			logger.With(log.Fields{"repo": r.GetName()}).Debugf("queue request")
			if err := q.Publish(j); err != nil {
				return err
			}
		}

		if r.NextPage == 0 {
			break
		}

		opts.Page = r.NextPage
	}

	logger.Infof("finished to publish queue jobs")

	return nil
}

func (s *RepositorySyncer) Sync(owner, name string) error {
	repository, _, err := s.c.Repositories.Get(context.TODO(), owner, name)
	if err != nil {
		return err
	}

	record, err := s.s.FindOne(models.NewRepositoryQuery().
		Where(kallax.Eq(models.Schema.Repository.ID, repository.GetID())),
	)

	if record == nil {
		record = models.NewRepository()
		record.Repository = *repository

		return s.s.Insert(record)
	}

	record.Repository = *repository
	_, err = s.s.Update(record)
	return err

}
