import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "normalize.css";

import "./index.css";
import App from "./App";
import { APIProvider } from "./useAPIState";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root element");

createRoot(root).render(
  <StrictMode>
    <APIProvider>
      <App />
    </APIProvider>
  </StrictMode>
);
