load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "transaction",
    srcs = ["transactioncontext.go"],
    importpath = "compass.com/postgres/transaction",
    visibility = ["//visibility:public"],
    deps = ["@com_github_jmoiron_sqlx//:sqlx"],
)
