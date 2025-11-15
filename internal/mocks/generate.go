package mocks

//go:generate minimock -i PR-reviewer/internal/service.Service -o ./service_mock.go -n ServiceMock -p mocks
//go:generate minimock -i PR-reviewer/internal/logger.Logger -o ./logger_mock.go -n LoggerMock -p mocks
//go:generate minimock -i PR-reviewer/internal/repo.Repo -o ./repo_mock.go -n RepoMock -p mocks
