import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import { authedFetch } from '../auth/authedFetch';

const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
});

const ProjectsResponseSchema = z.object({
  projects: z.array(ProjectSchema),
});

export type Project = z.infer<typeof ProjectSchema>;

export const PROJECTS_KEY = ['projects'] as const;

export function useProjects(enabled = true) {
  return useQuery({
    queryKey: PROJECTS_KEY,
    queryFn: async () => {
      const res = await authedFetch('/api/v1/projects');
      if (!res.ok) throw new Error(`fetch projects failed: ${String(res.status)}`);
      return ProjectsResponseSchema.parse(await res.json()).projects;
    },
    enabled,
    staleTime: 60_000,
  });
}
