import axios from "axios";

const API_ENDPOINT = process.env.API_ENDPOINT;
const username = process.env.BASIC_AUTH_USER;
const password = process.env.BASIC_AUTH_PASS;

const axiosInstance = axios.create({
    baseURL: API_ENDPOINT,
    auth: {
        username,
        password
    }
});

export default axiosInstance