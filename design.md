# youtube-gir interface

The extension follows a quiet Apple-style look. These tokens are what `extension/src/common.css` and the YouTube panel implement.

## Tone

Soft surfaces, large radii, short fades, and one blue accent. The interface should feel like a small system panel, not a marketing page. Motion is short and disabled when the user prefers reduced motion.

## Type

`-apple-system`, `BlinkMacSystemFont`, `SF Pro Text`, `Segoe UI`, then `Tahoma` so Persian remains readable. Titles use a tight negative tracking. Body text is 13–14px. Secondary text is `#6e6e73` in light mode and `#98989d` in dark mode.

## Color

| Token | Light | Dark |
| --- | --- | --- |
| Background | `#f4f4f6` | `#1c1c1e` |
| Card | white at 78% opacity | `#2c2c2e` at 86% opacity |
| Text | `#1d1d1f` | `#f5f5f7` |
| Accent | `#007aff` | `#0a84ff` |
| Success | `#34c759` | `#34c759` |
| Danger | `#ff3b30` | `#ff3b30` |
| Warning | `#ff9f0a` | `#ff9f0a` |

Cards use a 18px radius, a hairline inner stroke, and a wide soft shadow. The mark is a blue gradient from `#64d2ff` to `#0a84ff`.

## Behavior

Light and dark follow `prefers-color-scheme`. On YouTube, the injected panel also follows the page `dark` attribute. Layout uses logical properties so Persian (`fa`) can flip to right-to-left without a second stylesheet. Strings live in `extension/src/i18n.js`. Add a language by adding one object there.
