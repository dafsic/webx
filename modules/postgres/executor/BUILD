load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "executor",
    srcs = [
        "executor.go",
        "methods.go",
        "mock.go",
        "module.go",
    ],
    importpath = "compass.com/postgres/executor",
    visibility = ["//visibility:public"],
    deps = [
        "//src/go/compass.com/app",
        "@com_github_jmoiron_sqlx//:sqlx",
        "@com_github_stretchr_testify//mock",
    ],
)
