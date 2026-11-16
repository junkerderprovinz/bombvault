import { readdirSync, readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import { describe, expect, it } from 'vitest'

/**
 * Hover moves up the surface ramp in the accent's hue. `--hover` keeps the
 * lightness of #353535 and is the hover for an element with no fill of its own;
 * on a surface2 element (#393939) it dims under the pointer. A surface2 element
 * hovers to `--hover-filled`, and a surface3 element to `--hover-raised`.
 *
 * It reads each string literal rather than each line, because a variant table
 * puts a fill-less ghost variant on the line next to a filled one. A class
 * list split across two literals joined with `+` is read as two and slips past.
 */

const here = dirname(fileURLToPath(import.meta.url))
const src = join(here, '..')

/** Each resting fill, and the hover it may take. */
const RAMP = [
  { filled: 'bg-carbon-surface2', hover: 'hover:bg-hoverFilled' },
  { filled: 'bg-carbon-surface3', hover: 'hover:bg-hoverRaised' },
]
/**
 * The hover for an element with no fill, wrong on anything filled. Anchored,
 * because `hover:bg-hoverRaised` contains `hover:bg-hover`.
 */
const WRONG = /hover:bg-hover(?![A-Za-z-])/

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

/** Every piece of text that is one class list, with where it starts. */
function classLists(text: string): [string, number][] {
  const pieces: [string, number][] = []
  for (const found of text.matchAll(QUOTED)) pieces.push([found[0], found.index])
  for (const found of text.matchAll(TEMPLATE)) {
    // QUOTED has already taken the strings inside `${…}`; the static text
    // around them is a class list of its own.
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
    // An empty file list would make the assertion below pass while checking
    // nothing.
    const files = sourceFiles(src)
    expect(files.length).toBeGreaterThan(20)
    expect(files.some((f) => readFileSync(f, 'utf8').includes(RAMP[0].filled))).toBe(true)
  })

  it('never hovers a filled element to the tone below its own', () => {
    // Each hit names the class it should carry instead, because that depends
    // on which tier the element is resting on.
    const found = offenders()
    expect(found, `hovers below its own tone at: ${found.join(', ')}`).toEqual([])
  })
})
