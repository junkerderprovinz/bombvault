import { readdirSync, readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import { describe, expect, it } from 'vitest'

/**
 * GlimStone rule 21: hover moves UP the surface ramp.
 *
 * `--carbon-hover` (#353535) is the hover fill for something carrying no fill
 * of its own. It sits BELOW `--carbon-surface2` (#393939), so putting it on an
 * element that is already filled with surface2 makes that element four units
 * DARKER under the pointer - it dims at the one moment somebody is looking
 * straight at it, which reads as no hover at all. Anything already filled with
 * surface2 hovers to `--carbon-surface3` (#525252) instead, and anything
 * filled with surface3 to `--carbon-hover-raised` (#6f6f6f), which was added
 * for exactly this: the ramp used to stop at surface3, so the neutral button
 * sitting ON it reached back down to `--carbon-hover` - a 29-unit drop.
 *
 * A test rather than a note, because the note existed: GlimStone's token table
 * has said "hover on surface2" beside `--carbon-surface3` since the beginning,
 * and thirteen places across three apps still got it wrong. Prose that has
 * already failed to stop a mistake does not stop it by being repeated.
 *
 * It looks inside each STRING LITERAL, not at lines. That distinction is the
 * whole accuracy of it: a variant table writes one class list per line, so
 *
 *   secondary: 'bg-carbon-surface2 … hover:bg-carbon-surface3',
 *   ghost:     'text-carbon-textSub hover:bg-carbon-hover …',
 *
 * has the fill and the hover on ADJACENT lines belonging to different
 * variants, and a line window flags the ghost variant, which has no fill at
 * all and is written exactly right. A first draft of this guard did read lines
 * and reported two such pairs in a sibling app on its first run.
 *
 * KNOWN LIMIT: a class list split across two literals joined with `+` is read
 * as two, so a fill in one half and a hover in the other slips past.
 */

const here = dirname(fileURLToPath(import.meta.url))
const src = join(here, '..')

/** Each resting fill, and the hover it is allowed to take (rule 21). */
const RAMP = [
  { filled: 'bg-carbon-surface2', hover: 'hover:bg-carbon-surface3' },
  { filled: 'bg-carbon-surface3', hover: 'hover:bg-carbon-hoverRaised' },
]
/**
 * The hover for something with NO fill of its own, wrong on anything filled.
 *
 * Anchored, because `hover:bg-carbon-hoverRaised` CONTAINS
 * `hover:bg-carbon-hover`: a plain substring test reports every correctly
 * written surface3 control as the very mistake it avoids. It did, on the first
 * run after the third tier was added.
 */
const WRONG = /hover:bg-carbon-hover(?![A-Za-z-])/

function sourceFiles(dir: string): string[] {
  const found: string[] = []
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) found.push(...sourceFiles(path))
    else if (/\.tsx?$/.test(entry) && !entry.endsWith('.test.ts')) found.push(path)
  }
  return found
}

/** Plain quoted strings, and the static halves of template strings. */
const QUOTED = /'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"/g
const TEMPLATE = /`(?:[^`\\]|\\.)*`/g

/** Every piece of text that is ONE class list, with where it starts. */
function classLists(text: string): [string, number][] {
  const pieces: [string, number][] = []
  for (const found of text.matchAll(QUOTED)) pieces.push([found[0], found.index])
  for (const found of text.matchAll(TEMPLATE)) {
    // A template's `${…}` holds its own quoted strings, and QUOTED above has
    // already taken those one by one. What is left is the static text around
    // them, which is a class list of its own.
    let at = found.index
    for (const chunk of found[0].split(/\$\{[\s\S]*?\}/)) {
      pieces.push([chunk, at])
      at += chunk.length
    }
  }
  return pieces
}

/** Every `file:line` where a filled element hovers to the tone below its own. */
function offenders(): string[] {
  const hits: string[] = []
  for (const path of sourceFiles(src)) {
    const text = readFileSync(path, 'utf8')
    for (const [piece, at] of classLists(text)) {
      if (!WRONG.test(piece)) continue
      const tier = RAMP.find((t) => piece.includes(t.filled))
      if (!tier) continue
      const line = text.slice(0, at).split('\n').length
      hits.push(`${path.slice(src.length + 1)}:${line} (${tier.hover})`)
    }
  }
  return hits.sort()
}

describe('hover ramp', () => {
  it('reads the source at all', () => {
    // Guards the guard: a rename that empties this list would make the
    // assertion below pass forever while checking nothing.
    const files = sourceFiles(src)
    expect(files.length).toBeGreaterThan(20)
    expect(files.some((f) => readFileSync(f, 'utf8').includes(RAMP[0].filled))).toBe(true)
  })

  it('never hovers a filled element to the tone below its own', () => {
    // Each hit names the class it should carry instead, because that depends
    // on which tier the element is resting on.
    expect(offenders(), `hovers below its own tone at: ${offenders().join(', ')}`).toEqual([])
  })
})
