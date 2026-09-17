Analyze the provided pixel-art sprite for the purpose of matching it to high-resolution creature images.

The sprite is an intentionally tiny 8×8 pixel-art icon, together with a version upscaled to 128x128.

Your task is NOT to identify the exact original creature or guess what the artist intended.

Instead, extract the visual information that can actually be communicated by the 8×8 pixels, followed by cautious interpretations of what those visual structures might represent.

Think like a pixel-art analyst:

- What does the silhouette look like?
- What major shapes and protrusions are visible?
- Is it humanoid, animal-like, skeletal, winged, etc.?
- Which features are visually prominent enough to survive at 8×8?
- What creature concepts could this icon plausibly evoke?

Analyze the sprite primarily at its native 8×8 resolution. An enlarged nearest-neighbor version may be provided for inspection, but do not treat interpolation or enlargement artifacts as additional information.

Return ONLY valid JSON using this schema:

{
  "id": "",
  "perceptual": {
    "silhouette": [],
    "shape": [],
    "orientation": [],
    "pose": [],
    "colors": [],
    "major_regions": []
  },
  "interpreted_features": {
    "body_plan": [],
    "possible_features": []
  },
  "iconography": {
    "semantic_concepts": [],
    "visual_symbols": [],
    "priority_features": []
  }
}

FIELD DEFINITIONS

id:
The supplied sprite identifier, if one is provided.

perceptual:
Describe what is actually visible in the pixels without assigning creature identity.

silhouette:
Describe the overall outline and its most prominent features.

Examples:
["roughly humanoid silhouette", "wide upper body", "two upward protrusions", "narrow lower body"]
["compact quadrupedal silhouette", "large central body", "four short projections"]
["tall narrow silhouette", "large circular head", "extended arm-like projection"]

shape:
Describe major geometric forms.

Examples:
["large central mass", "small head-like mass", "long thin extension", "rounded upper region"]

orientation:
Describe the apparent facing/orientation.

Examples:
["front-facing", "side-facing", "facing left", "facing right", "ambiguous"]

pose:
Describe the apparent pose or action.

Examples:
["standing", "crouching", "flying", "arms spread", "four-legged stance", "ambiguous"]

colors:
Describe only the dominant visible color relationships.

Examples:
["mostly dark gray", "red central region", "light face against dark body"]

Do not give colors disproportionate importance.

major_regions:
Describe the largest visually distinct regions.

Examples:
["dark body with light head", "two light protrusions above body", "bright weapon-like region"]

interpreted_features:
These are interpretations rather than objective observations.

body_plan:
Infer the most plausible broad anatomical structure, but remain conservative.

Examples:
["humanoid", "bipedal", "quadrupedal", "winged humanoid", "serpentine", "skeletal humanoid"]

If uncertain, include multiple plausible interpretations or leave the field empty.

possible_features:
Features that the pixel structure could represent.

Examples:
["upward protrusions could be horns", "side projections could be wings", "long extension could be a tail or weapon"]

IMPORTANT:
Use uncertainty explicitly.

GOOD:
["two upward protrusions that could represent horns"]

BAD:
["large demon horns"]

iconography:
Describe what the icon is likely to communicate to a viewer, rather than what the original creature necessarily was.

semantic_concepts:
Broad creature concepts the icon could plausibly evoke.

Examples:
["humanoid monster", "demon", "undead", "beast", "bird-like creature"]

Use broad concepts when the pixels do not support a specific identification.

visual_symbols:
Recognizable symbolic elements.

Examples:
["horns", "skull-like head", "wings", "large weapon", "long tail", "claws"]

priority_features:
The 2–5 visual characteristics that are most important for matching this icon to a reference creature.

Prioritize features that are visually distinctive at 8×8.

Examples:
["large horns", "broad humanoid silhouette", "dark body", "small bright face"]

RULES

1. Analyze the actual pixels, not the presumed identity of the sprite.

2. Separate observation from interpretation.

3. Do not confidently identify a creature unless the 8×8 image provides strong visual evidence.

4. Do not invent anatomy that cannot reasonably be inferred from the pixels.

5. Treat ambiguous structures as ambiguous.

6. Focus heavily on silhouette, major shapes, protrusions, body plan, and iconographic symbols.

7. Ignore tiny decorative details that would not matter at 8×8.

8. Do not over-weight color. Shape and iconography are more important.

9. A sprite may represent something abstract rather than anatomically accurate. Describe the visual abstraction rather than forcing it into a realistic creature.

10. The goal is to make this representation useful for matching the sprite against a completely different high-resolution image.

11. Keep every array concise. Normally use 1–6 items per field.

12. Do not include explanations outside the JSON.

Return only the JSON object.
