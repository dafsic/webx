load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "temporarytables",
    srcs = [
        "create_temporary_table.go",
        "temporarytables.go",
    ],
    importpath = "compass.com/postgres/temporarytables",
    visibility = ["//visibility:public"],
    deps = [
        "//src/go/compass.com/postgres/temporarytables/errors",
        "//src/go/compass.com/postgres/transaction",
    ],
)

go_test(
    name = "temporarytables_test",
    srcs = ["create_temporary_table_test.go"],
    deps = [
        ":temporarytables",
        "//src/go/compass.com/mocks/postgres/transaction",
        "//src/go/compass.com/postgres/temporarytables/errors",
        "@com_github_pkg_errors//:errors",
        "@com_github_stretchr_testify//mock",
        "@com_github_stretchr_testify//suite",
    ],
)
