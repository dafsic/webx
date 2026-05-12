load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "expression",
    srcs = ["array_expressions.go"],
    importpath = "compass.com/postgres/expression",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_lib_pq//:pq",
        "@com_github_masterminds_squirrel//:squirrel",
    ],
)

go_test(
    name = "expression_test",
    srcs = ["array_expressions_test.go"],
    deps = [
        ":expression",
        "@com_github_lib_pq//:pq",
        "@com_github_stretchr_testify//suite",
    ],
)
