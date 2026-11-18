"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deleteProjectQuestion,
  deleteProjectQuestionAnswer,
  getWorkspaceProjectQuestions,
  patchProjectQuestion,
  postWorkspaceProjectQuestions,
} from "@/lib/api/endpoints";
import { qk } from "@/lib/query/keys";

export const useProjectQuestions = (ws: string, projectId: string) =>
  useQuery({ queryKey: qk.projectQuestions(ws, projectId), queryFn: async () => (await getWorkspaceProjectQuestions(ws, projectId)) ?? [], enabled: !!ws && !!projectId });

export function useQuestionMutations(ws: string, projectId: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: qk.projectQuestions(ws, projectId) });
  return {
    create: useMutation({ mutationFn: (question: string) => postWorkspaceProjectQuestions(ws, projectId, { question }), onSuccess: refresh }),
    update: useMutation({ mutationFn: (v: { id: string; question: string }) => patchProjectQuestion(v.id, { question: v.question }), onSuccess: refresh }),
    clearAnswer: useMutation({ mutationFn: (id: string) => deleteProjectQuestionAnswer(id), onSuccess: refresh }),
    remove: useMutation({ mutationFn: (id: string) => deleteProjectQuestion(id), onSuccess: refresh }),
  };
}
