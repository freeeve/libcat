# Editor links: friendly labels and cover thumbnails

The instance Links field showed bare 856 $u URLs -- four raw OverDrive
URLs per record with no hint which is the title page, the sample, or the
cover art.

The source 856s do carry that context ($3 Image/Thumbnail/Excerpt, $z
public notes), but the libcodex crosswalk keeps only $u
(Instance.ElectronicLocator is []string), so the labels never reach the
grain. Filed libcodex tasks/086 for a structured locator; until then the
editor derives display hints from the URL shape client-side:

- `lib/links.ts` `linkInfo(url)`: link.overdrive.com -> "OverDrive title
  page", samples.overdrive.com -> "Sample (excerpt)", od-cdn.com
  ImageType-100/200 -> "Cover image"/"Cover thumbnail", generic image
  extensions -> "Image"; unrecognized URLs pass through unlabeled.
- ProfileForm's links field renders labeled links as bold label + muted
  host (full URL in the tooltip/href), and image links show an inline
  lazy-loaded thumbnail.

Covered by lib/links.test.ts (URL classification) and
ProfileForm.links.test.ts (rendered DOM: labels, thumbnail img, plain
fallback). Once libcodex carries $3/$z, replace the heuristic with real
labels from the grain.
