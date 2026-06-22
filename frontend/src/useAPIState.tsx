import React, {
  useReducer,
  useCallback,
  useEffect,
  PropsWithChildren,
} from "react";

import { fetchImages, fetchScores, Image, Score } from "./api";

type APIState = {
  images: Image[];
  scores: Score[];
  error?: string;
  refreshImages: () => Promise<void>;
  refreshScores: () => Promise<void>;
};

type SetScoresAction = { type: "setScores"; payload: Score[] };
type SetImagesAction = { type: "setImages"; payload: Image[] };
type SetErrorAction = { type: "setError"; payload: string };
type APIAction = SetImagesAction | SetScoresAction | SetErrorAction;

type InnerState = { images: Image[]; scores: Score[]; error?: string };

const APIContext = React.createContext<APIState | undefined>(undefined);

const reducer = (state: InnerState, action: APIAction): InnerState => {
  switch (action.type) {
    case "setImages":
      return { ...state, error: undefined, images: action.payload };
    case "setScores":
      return { ...state, error: undefined, scores: action.payload };
    case "setError":
      return { ...state, error: action.payload };
    default:
      return state;
  }
};

const APIProvider = ({ children }: PropsWithChildren) => {
  const [state, dispatch] = useReducer(reducer, { images: [], scores: [] });

  const refreshImages = useCallback(async () => {
    try {
      dispatch({ type: "setImages", payload: await fetchImages("") });
    } catch (e) {
      dispatch({ type: "setError", payload: String(e) });
    }
  }, []);

  const refreshScores = useCallback(async () => {
    try {
      dispatch({ type: "setScores", payload: await fetchScores("") });
    } catch (e) {
      dispatch({ type: "setError", payload: String(e) });
    }
  }, []);

  useEffect(() => {
    refreshImages();
    refreshScores();
  }, [refreshImages, refreshScores]);

  return (
    <APIContext.Provider
      value={{ ...state, refreshImages, refreshScores }}
    >
      {children}
    </APIContext.Provider>
  );
};

const useAPIState = (): APIState => {
  const context = React.useContext(APIContext);
  if (context === undefined) {
    throw new Error("useAPIState must be used within an APIProvider");
  }
  return context;
};

export { useAPIState as default, APIProvider };
