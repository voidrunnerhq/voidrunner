window.BENCHMARK_DATA = {
  "lastUpdate": 1752508376912,
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
          "id": "d1021445fcebaf4c3896eb3c5d2bdc714051ddc3",
          "message": "feat(executor): implement Docker client integration with security controls\n\nImplement complete Docker client integration for secure task execution:\n- Docker client with connection health checks and validation\n- Comprehensive resource limits (memory, CPU, PIDs) with security caps\n- Network isolation and non-root execution (UID:GID 1000:1000)\n- Security profiles with Seccomp/AppArmor support\n- Multi-language script validation (Python, Bash, JavaScript, Go)\n- Automatic container cleanup and lifecycle management\n- Integration with API server and task execution service\n\nFixes #9\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-13T01:54:41+08:00",
          "tree_id": "5e5714427a98b2eb3f762cac4f22ea174f0cda4a",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/d1021445fcebaf4c3896eb3c5d2bdc714051ddc3"
        },
        "date": 1752342962581,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3326,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "332047 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3326,
            "unit": "ns/op",
            "extra": "332047 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "332047 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "332047 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 2952,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "376844 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 2952,
            "unit": "ns/op",
            "extra": "376844 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "376844 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "376844 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 182648,
            "unit": "ns/op\t  106230 B/op\t    1458 allocs/op",
            "extra": "6534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 182648,
            "unit": "ns/op",
            "extra": "6534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106230,
            "unit": "B/op",
            "extra": "6534 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "6534 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 135.9,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "7646632 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 135.9,
            "unit": "ns/op",
            "extra": "7646632 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "7646632 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "7646632 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 403.5,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2955937 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 403.5,
            "unit": "ns/op",
            "extra": "2955937 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2955937 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2955937 times\n4 procs"
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
          "id": "fad4bf37b906e74bd7bf3e3883e2b0b5b0ed5202",
          "message": "fix: resolve formatting issues and improve task execution error handling\n\n- Fix gofmt formatting issue in e2e_test.go (remove trailing blank line)\n- Add defensive nil check in task execution handler to prevent panics\n- Update task status to running when execution starts (instead of pending)\n- Add nil check for queue manager in task execution service for test compatibility\n- Implement complete mock queue interfaces for integration tests\n- Improve error handling and logging consistency across execution workflow\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-14T11:15:50+08:00",
          "tree_id": "f0d25a09a188e407c7805683a81318f741fa2003",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/fad4bf37b906e74bd7bf3e3883e2b0b5b0ed5202"
        },
        "date": 1752463030238,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3313,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "320336 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3313,
            "unit": "ns/op",
            "extra": "320336 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "320336 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "320336 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 2942,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "343791 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 2942,
            "unit": "ns/op",
            "extra": "343791 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "343791 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "343791 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 186203,
            "unit": "ns/op\t  106299 B/op\t    1458 allocs/op",
            "extra": "5677 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 186203,
            "unit": "ns/op",
            "extra": "5677 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106299,
            "unit": "B/op",
            "extra": "5677 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "5677 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 136.7,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8889310 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 136.7,
            "unit": "ns/op",
            "extra": "8889310 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8889310 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8889310 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 421.9,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2863992 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 421.9,
            "unit": "ns/op",
            "extra": "2863992 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2863992 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2863992 times\n4 procs"
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
          "id": "95504ddaa3a6d691fc240e77e8864be98e9bca08",
          "message": "fix: resolve make dev-up failure and improve development environment\n\n- feat: upgrade PostgreSQL from 15-alpine to 17-alpine to resolve version incompatibility\n- fix: remove unnecessary init-db.sql volume mount (migrations handle schema)\n- fix: remove deprecated Docker Compose version fields\n- fix: add /app/tmp directory and Go module cache permissions in Dockerfile\n- feat: generate missing Swagger API documentation\n- fix: improve Redis authentication handling for development environment\n- fix: resolve Go formatting issues\n\nFixes the primary `make dev-up` failure that prevented development environment startup.\nPostgreSQL, Redis, and API containers now start successfully with proper health checks.\nDatabase migrations work correctly with proper environment variable configuration.\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-14T17:55:55+08:00",
          "tree_id": "48e9963bf2563c5d34e1e3f8958f6d7983d832e1",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/95504ddaa3a6d691fc240e77e8864be98e9bca08"
        },
        "date": 1752487001861,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 4020,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "292292 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 4020,
            "unit": "ns/op",
            "extra": "292292 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "292292 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "292292 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 4209,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "302427 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 4209,
            "unit": "ns/op",
            "extra": "302427 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "302427 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "302427 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 188983,
            "unit": "ns/op\t  106237 B/op\t    1458 allocs/op",
            "extra": "6668 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 188983,
            "unit": "ns/op",
            "extra": "6668 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106237,
            "unit": "B/op",
            "extra": "6668 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "6668 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 138.7,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8596513 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 138.7,
            "unit": "ns/op",
            "extra": "8596513 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8596513 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8596513 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 408.4,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2909083 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 408.4,
            "unit": "ns/op",
            "extra": "2909083 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2909083 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2909083 times\n4 procs"
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
          "id": "2cad839af8bb62a2e8c92a4173f79a133b691ebd",
          "message": "docker(deps): Bump alpine from 3.19 to 3.22\n\nBumps alpine from 3.19 to 3.22.\n\n---\nupdated-dependencies:\n- dependency-name: alpine\n  dependency-version: '3.22'\n  dependency-type: direct:production\n  update-type: version-update:semver-minor\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2025-07-14T23:15:08+08:00",
          "tree_id": "383f0d43e503dce726167279d2fda4283df51516",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/2cad839af8bb62a2e8c92a4173f79a133b691ebd"
        },
        "date": 1752506159380,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3347,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "354315 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3347,
            "unit": "ns/op",
            "extra": "354315 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "354315 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "354315 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 2990,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "360127 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 2990,
            "unit": "ns/op",
            "extra": "360127 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "360127 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "360127 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 182044,
            "unit": "ns/op\t  106209 B/op\t    1458 allocs/op",
            "extra": "6529 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 182044,
            "unit": "ns/op",
            "extra": "6529 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106209,
            "unit": "B/op",
            "extra": "6529 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "6529 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 136.1,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8755515 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 136.1,
            "unit": "ns/op",
            "extra": "8755515 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8755515 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8755515 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 402.8,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "3003411 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 402.8,
            "unit": "ns/op",
            "extra": "3003411 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "3003411 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "3003411 times\n4 procs"
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
          "id": "3291b43b781482d1caa7d5912b7775ee196e115b",
          "message": "deps(deps): Bump golang.org/x/crypto from 0.39.0 to 0.40.0\n\nBumps [golang.org/x/crypto](https://github.com/golang/crypto) from 0.39.0 to 0.40.0.\n- [Commits](https://github.com/golang/crypto/compare/v0.39.0...v0.40.0)\n\n---\nupdated-dependencies:\n- dependency-name: golang.org/x/crypto\n  dependency-version: 0.40.0\n  dependency-type: direct:production\n  update-type: version-update:semver-minor\n...\n\nSigned-off-by: dependabot[bot] <support@github.com>",
          "timestamp": "2025-07-14T23:15:25+08:00",
          "tree_id": "3a75cf5e0d1f0ca58d3bbd8997f73035ed63501b",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/3291b43b781482d1caa7d5912b7775ee196e115b"
        },
        "date": 1752506214940,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 4241,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "320354 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 4241,
            "unit": "ns/op",
            "extra": "320354 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "320354 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "320354 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 3016,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "354234 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 3016,
            "unit": "ns/op",
            "extra": "354234 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "354234 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "354234 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 187696,
            "unit": "ns/op\t  106253 B/op\t    1458 allocs/op",
            "extra": "7770 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 187696,
            "unit": "ns/op",
            "extra": "7770 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106253,
            "unit": "B/op",
            "extra": "7770 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "7770 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 136.7,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8744680 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 136.7,
            "unit": "ns/op",
            "extra": "8744680 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8744680 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8744680 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 404.6,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2948431 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 404.6,
            "unit": "ns/op",
            "extra": "2948431 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2948431 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2948431 times\n4 procs"
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
          "id": "1f0d035ab4d16e4fddb5d50a8dca1266ffe7edad",
          "message": "fix(dev): resolve Redis connection issue in development environment\n\n- Add missing REDIS_HOST environment variable to API container\n- Fix network configuration mismatch between compose files\n- Ensure consistent use of voidrunner-backend network\n- API now successfully connects to Redis service\n\nThe issue was caused by the API container trying to connect to Redis\nat localhost instead of the Docker service name 'redis'. This fix\nensures proper service discovery within the Docker network.\n\nFixes Redis connection errors in 'make dev-up' command.\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
          "timestamp": "2025-07-14T23:51:45+08:00",
          "tree_id": "795f79e492e5ffac101cf773b1ff7002aafdf26a",
          "url": "https://github.com/voidrunnerhq/voidrunner/commit/1f0d035ab4d16e4fddb5d50a8dca1266ffe7edad"
        },
        "date": 1752508376443,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex",
            "value": 3392,
            "unit": "ns/op\t   16897 B/op\t      29 allocs/op",
            "extra": "331400 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - ns/op",
            "value": 3392,
            "unit": "ns/op",
            "extra": "331400 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - B/op",
            "value": 16897,
            "unit": "B/op",
            "extra": "331400 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_GetAPIIndex - allocs/op",
            "value": 29,
            "unit": "allocs/op",
            "extra": "331400 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI",
            "value": 2995,
            "unit": "ns/op\t    7449 B/op\t      33 allocs/op",
            "extra": "376120 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - ns/op",
            "value": 2995,
            "unit": "ns/op",
            "extra": "376120 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - B/op",
            "value": 7449,
            "unit": "B/op",
            "extra": "376120 times\n4 procs"
          },
          {
            "name": "BenchmarkDocsHandler_RedirectToSwaggerUI - allocs/op",
            "value": 33,
            "unit": "allocs/op",
            "extra": "376120 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup",
            "value": 187216,
            "unit": "ns/op\t  106256 B/op\t    1458 allocs/op",
            "extra": "6367 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - ns/op",
            "value": 187216,
            "unit": "ns/op",
            "extra": "6367 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - B/op",
            "value": 106256,
            "unit": "B/op",
            "extra": "6367 times\n4 procs"
          },
          {
            "name": "BenchmarkSetup - allocs/op",
            "value": 1458,
            "unit": "allocs/op",
            "extra": "6367 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New",
            "value": 135.5,
            "unit": "ns/op\t     184 B/op\t       5 allocs/op",
            "extra": "8741970 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - ns/op",
            "value": 135.5,
            "unit": "ns/op",
            "extra": "8741970 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - B/op",
            "value": 184,
            "unit": "B/op",
            "extra": "8741970 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_New - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8741970 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID",
            "value": 403.9,
            "unit": "ns/op\t     288 B/op\t       8 allocs/op",
            "extra": "2964346 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - ns/op",
            "value": 403.9,
            "unit": "ns/op",
            "extra": "2964346 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - B/op",
            "value": 288,
            "unit": "B/op",
            "extra": "2964346 times\n4 procs"
          },
          {
            "name": "BenchmarkLogger_WithRequestID - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "2964346 times\n4 procs"
          }
        ]
      }
    ]
  }
}