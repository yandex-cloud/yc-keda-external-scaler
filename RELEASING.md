# Releasing

Releases are created only from stable version tags whose commits are contained
in `master`. The scaler image is published manually to the existing Yandex
Container Registry repository; GitHub Actions never builds or pushes images.

For `v1.3.0`:

1. Run `make verify`.
2. Authenticate Docker for `cr.yandex` using the established `sol` workflow.
3. Build and push the immutable multi-platform image:

   ```bash
   docker buildx build \
     --platform linux/amd64,linux/arm64 \
     --tag cr.yandex/sol/keda/yc-keda-external-scaler:v1.3.0 \
     --push .
   ```

4. Verify the image is publicly readable and contains both platforms:

   ```bash
   docker buildx imagetools inspect \
     cr.yandex/sol/keda/yc-keda-external-scaler:v1.3.0
   ```

5. Install the packaged chart in a test cluster and complete the smoke test.
6. Commit and push the reviewed changes to `master`.
7. Create and push the immutable tag:

   ```bash
   git tag -s v1.3.0 -m "yc-keda-external-scaler v1.3.0"
   git push origin v1.3.0
   ```

The tag workflow verifies the public image, packages the chart, creates
`checksums.txt`, and publishes both files on the GitHub Release. Never move or
recreate a published version tag; publish a new patch version instead.
