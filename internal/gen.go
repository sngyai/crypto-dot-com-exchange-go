package internal

import _ "github.com/golang/mock/mockgen/model"

//go:generate mockgen -destination=./mocks/id/generator_mock.gen.go -package=id_mocks github.com/cshep4/go-cryptocom/internal/id IDGenerator
//go:generate mockgen -destination=./mocks/signature/generator_mock.gen.go -package=signature_mocks github.com/cshep4/go-cryptocom/internal/auth SignatureGenerator
