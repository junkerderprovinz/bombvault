// What the check card's tests share: picking an action from a section's menu.
import { act, fireEvent, screen, within } from "@testing-library/react";
import { en } from "../../lib/i18n";

export async function pickAction(domainName: string, action: string): Promise<void> {
  const row = await screen.findByText((_, el) => el?.getAttribute("data-domain") === domainName);
  await act(async () => {
    fireEvent.click(within(row).getByRole("button", { name: en["nav.more"] }));
  });
  await act(async () => {
    fireEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name: action }));
  });
}
