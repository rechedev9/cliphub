import { createRoot } from "react-dom/client";

import { Bridge } from "../bridge.ts";
import { App } from "./app.tsx";

const node = document.getElementById("root");
if (node) createRoot(node).render(<App bridge={new Bridge()} />);
