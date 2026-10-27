// SelectField runs on Zag's select machine, which starts after the first
// render and answers every event in a microtask, and some of those queue
// more. A test that drives one waits for a task after each step, or it reads
// the page before the machine has moved.
import { act, fireEvent, screen } from "@testing-library/react";

/** Runs `step`, then lets the machine catch up. */
export async function settle(step: () => void = () => {}): Promise<void> {
  await act(async () => {
    step();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

/** Opens the list of a SelectField, given its trigger. */
export async function openSelect(trigger: HTMLElement): Promise<void> {
  await settle();
  await settle(() => fireEvent.click(trigger));
}

/** Opens the list of a SelectField and picks an option by its name. */
export async function pickOption(trigger: HTMLElement, option: string | RegExp): Promise<void> {
  await openSelect(trigger);
  await settle(() => fireEvent.click(screen.getByRole("option", { name: option })));
}
