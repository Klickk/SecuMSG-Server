import { Route, Routes } from "react-router-dom";
import { AuthPage } from "./components/AuthPage";
import { DeviceRegisterForm } from "./components/DeviceRegister";
import { MessagingPage } from "./components/MessagingPage";

const App = () => {
  return (
    <div>
      <Routes>
        <Route path="/" Component={AuthPage} />
        <Route path="/dRegister" Component={DeviceRegisterForm} />
        <Route path="/messages" Component={MessagingPage} />
      </Routes>
    </div>
  );
};

export default App;
