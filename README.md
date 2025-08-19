## Usage:

GitLab CI: (non-blocking job, using *.cover.txt from tests job)
```
test-coverage:
  image: golang:1.22.12
  stage: tests
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
  allow_failure: true
  needs:
    - tests
  before_script:
    - go mod download
    - go install github.com/akalmyk/diffcover/cmd/diffcover@vlatest
  script:
    - git fetch origin $CI_MERGE_REQUEST_TARGET_BRANCH_NAME
    - git fetch origin $CI_MERGE_REQUEST_SOURCE_BRANCH_NAME
    - git diff origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME...origin/$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME > diff.patch
    - "echo mode: set > coverage.out"
    - cat *.cover.txt | grep -v mode:|sort -r|awk '!seen[$1]++' >> coverage.out
    - diffcover diff.patch coverage.out diff_coverage.out 80 || true
    - if [ ! -f diff_coverage.out ]; then echo "diff_coverage.out not found" >&2; exit 1; fi
    - go tool cover -html=diff_coverage.out -o $CI_PROJECT_DIR/diff_coverage.html
    - if [ ! -f diff_coverage.html ]; then echo "diff_coverage.html not found" >&2; exit 1; fi
    - if [ ! -f diffcover.failed ]; then exit 0; else echo "❌ diff coverage < 80%"; exit 1; fi
  artifacts:
    when: always
    paths:
      - diff_coverage.html
    expire_in: 1 week
```
