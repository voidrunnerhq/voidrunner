window.BENCHMARK_DATA = {
  "lastUpdate": 1752199737645,
  "repoUrl": "https://github.com/voidrunnerhq/voidrunner",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "starbops@zespre.com",
            "name": "Zespre Schmidt",
            "username": "starbops"
          },
          "committer": {
            "email": "starbops@hey.com",
            "name": "Zespre Chang",
            "username": "starbops"
          },
          "distinct": true,
          "id": "38dff3d45569ae0c357908edff3cf108408cd15d",
          "message": "ci: adopt codeowners instead of reviewers in dependabot\n\nSigned-off-by: Zespre Schmidt <starbops@zespre.com>",
          "timestamp": "2025-07-11T00:24:34+08:00",
          "tree_id": "985f07d42b6a0d81e6bd54a1985b2ac71f129696",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/38dff3d45569ae0c357908edff3cf108408cd15d"
        },
        "date": 1752164749281,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3444,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "312259 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3444,
            "unit": "ns/op",
            "extra": "312259 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "312259 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "312259 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 3024,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "375122 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 3024,
            "unit": "ns/op",
            "extra": "375122 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "375122 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "375122 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 186386,
            "unit": "ns/op\t  105961 B/op\t    1456 allocs/op",
            "extra": "7048 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 186386,
            "unit": "ns/op",
            "extra": "7048 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 105961,
            "unit": "B/op",
            "extra": "7048 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1456,
            "unit": "allocs/op",
            "extra": "7048 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 137.5,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8848882 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 137.5,
            "unit": "ns/op",
            "extra": "8848882 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8848882 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8848882 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 400.9,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2982200 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 400.9,
            "unit": "ns/op",
            "extra": "2982200 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2982200 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2982200 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "starbops@zespre.com",
            "name": "Zespre Schmidt",
            "username": "starbops"
          },
          "committer": {
            "email": "starbops@hey.com",
            "name": "Zespre Chang",
            "username": "starbops"
          },
          "distinct": true,
          "id": "77572372883d144db639e59bf1dfa3e9b56b49a1",
          "message": "fix(docker): update Go version to 1.24.4 for consistency\n\n- Update Dockerfile to use golang:1.24.4-alpine instead of golang:1.24-alpine\n- Ensures complete consistency across all documentation and build files\n- Aligns with go.mod, CI workflow, and documentation requirements\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-10T23:48:29+08:00",
          "tree_id": "fcb56d34c049a39e61bc8613d9c7ade8bef0957e",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/77572372883d144db639e59bf1dfa3e9b56b49a1"
        },
        "date": 1752164779469,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3507,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "295208 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3507,
            "unit": "ns/op",
            "extra": "295208 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "295208 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "295208 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 3292,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "328923 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 3292,
            "unit": "ns/op",
            "extra": "328923 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "328923 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "328923 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 195494,
            "unit": "ns/op\t  105982 B/op\t    1456 allocs/op",
            "extra": "7080 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 195494,
            "unit": "ns/op",
            "extra": "7080 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 105982,
            "unit": "B/op",
            "extra": "7080 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1456,
            "unit": "allocs/op",
            "extra": "7080 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 135.6,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8831404 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 135.6,
            "unit": "ns/op",
            "extra": "8831404 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8831404 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8831404 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 401.5,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "3006612 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 401.5,
            "unit": "ns/op",
            "extra": "3006612 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "3006612 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "3006612 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "49699333+dependabot[bot]@users.noreply.github.com",
            "name": "dependabot[bot]",
            "username": "dependabot[bot]"
          },
          "committer": {
            "email": "starbops@hey.com",
            "name": "Zespre Chang",
            "username": "starbops"
          },
          "distinct": true,
          "id": "d2ffe9a7498bf7e18ef3e6beccc31069e149b958",
          "message": "docker(deps): Bump golang from 1.24.4-alpine to 1.24.5-alpine\n\nBumps golang from 1.24.4-alpine to 1.24.5-alpine.\n\n---\nupdated-dependencies:\n- dependency-name: golang\n  dependency-version: 1.24.5-alpine\n  dependency-type: direct:production\n  update-type: version-update:semver-patch\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2025-07-11T00:35:30+08:00",
          "tree_id": "3528812eb9b3086910653e50f098eda0a5fee69f",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/d2ffe9a7498bf7e18ef3e6beccc31069e149b958"
        },
        "date": 1752165402550,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3511,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "300412 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3511,
            "unit": "ns/op",
            "extra": "300412 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "300412 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "300412 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 3402,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "350808 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 3402,
            "unit": "ns/op",
            "extra": "350808 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "350808 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "350808 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 178989,
            "unit": "ns/op\t  105973 B/op\t    1456 allocs/op",
            "extra": "6222 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 178989,
            "unit": "ns/op",
            "extra": "6222 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 105973,
            "unit": "B/op",
            "extra": "6222 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1456,
            "unit": "allocs/op",
            "extra": "6222 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 137,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8764185 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 137,
            "unit": "ns/op",
            "extra": "8764185 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8764185 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8764185 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 412.4,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2975174 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 412.4,
            "unit": "ns/op",
            "extra": "2975174 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2975174 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2975174 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "starbops@zespre.com",
            "name": "Zespre Schmidt",
            "username": "starbops"
          },
          "committer": {
            "email": "starbops@hey.com",
            "name": "Zespre Chang",
            "username": "starbops"
          },
          "distinct": true,
          "id": "53cbc4cc1f794a253814f3bad18aece465bc626b",
          "message": "fix(ci): add contents write permissions for docs and performance jobs\n\n- Add contents: write to docs job for GitHub Pages deployment to gh-pages branch\n- Add contents: write to performance job for benchmark data storage to gh-pages branch\n- Fixes permission denied errors when pushing to repository\n- Maintains security with minimal required permissions for each job function\n\nFixes run #16209410222 failures.\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-11T10:07:40+08:00",
          "tree_id": "6bfc2cb3d1fe6c8e87eadcd7837889c22f381cd5",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/53cbc4cc1f794a253814f3bad18aece465bc626b"
        },
        "date": 1752199736803,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3765,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "269162 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3765,
            "unit": "ns/op",
            "extra": "269162 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "269162 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "269162 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 3026,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "364616 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 3026,
            "unit": "ns/op",
            "extra": "364616 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "364616 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "364616 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 184828,
            "unit": "ns/op\t  106060 B/op\t    1456 allocs/op",
            "extra": "5534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 184828,
            "unit": "ns/op",
            "extra": "5534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106060,
            "unit": "B/op",
            "extra": "5534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1456,
            "unit": "allocs/op",
            "extra": "5534 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 135.7,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8904964 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 135.7,
            "unit": "ns/op",
            "extra": "8904964 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8904964 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8904964 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 418.8,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2879461 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 418.8,
            "unit": "ns/op",
            "extra": "2879461 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2879461 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2879461 times\n4 procs"
          }
        ]
      }
    ]
  }
}