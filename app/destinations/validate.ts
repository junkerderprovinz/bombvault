import { z } from "zod";

export const destinationFormSchema = z.object({
  name: z.string().min(1, "Name is required"),
  repoPath: z.string().min(1, "Repository path is required"),
  password: z.string().min(1, "Password is required"),
});

export type DestinationFormInput = z.infer<typeof destinationFormSchema>;

/** Parse and validate raw form data. Throws a ZodError on invalid input. */
export function parseDestinationForm(input: unknown): DestinationFormInput {
  return destinationFormSchema.parse(input);
}
